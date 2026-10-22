package service

import (
	"context"
	"errors"
	"strings"

	"vane-common/pkg/dataaccess"
	"vane-common/pkg/dataaccess/base"
)

type iamUserRow struct {
	base.BaseModel
	UserKey  string `gorm:"column:user_key"`
	TenantID string `gorm:"column:tenant_id"`
}

func (*iamUserRow) TableName() string { return "iam_users" }

func (s *RelayService) clearDefaultEmployees(ctx context.Context, tenantID, exceptID string) error {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil
	}
	items, err := s.employees.List(ctx,
		dataaccess.Eq("tenant_id", tenantID),
		dataaccess.Eq("is_default", true),
		dataaccess.IsNull("deleted_at"),
	)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item == nil || item.EmployeeKey == exceptID || !item.IsDefault {
			continue
		}
		item.IsDefault = false
		if err := s.employees.Update(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func (s *RelayService) findEmployeeByKey(ctx context.Context, employeeID string) (*AIEmployee, error) {
	item, err := s.employees.First(ctx,
		dataaccess.Eq("employee_key", strings.TrimSpace(employeeID)),
		dataaccess.IsNull("deleted_at"),
	)
	if err != nil {
		if errors.Is(err, dataaccess.ErrDataNotFound) {
			return nil, notFound("AI 客服不存在")
		}
		return nil, err
	}
	return item, nil
}

// resolveEmployee picks the running employee bound to the platform.
// blocked is true when a binding exists but that employee is not running,
// including a paused or deleted employee. The default employee is not used then.
func (s *RelayService) resolveEmployee(ctx context.Context, tenantID, platform string) (*AIEmployee, bool, error) {
	tenantID = strings.TrimSpace(tenantID)
	platform = normalizePlatform(platform)
	if tenantID == "" || platform == "" {
		return nil, false, nil
	}
	rows, err := s.bindings.List(ctx, dataaccess.Eq("tenant_id", tenantID), dataaccess.Eq("platform", platform))
	if err != nil {
		return nil, false, err
	}
	var running *AIEmployee
	blocked := false
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		if _, ok := seen[row.EmployeeKey]; ok {
			continue
		}
		seen[row.EmployeeKey] = struct{}{}
		item, err := s.findEmployeeByKey(ctx, row.EmployeeKey)
		if err != nil {
			var serviceErr *Error
			if errors.As(err, &serviceErr) && serviceErr.Status == 404 {
				blocked = true
				continue
			}
			return nil, false, err
		}
		if item.Status == employeeStatusRunning {
			if running == nil || (!item.CreateAt.IsZero() && item.CreateAt.Before(running.CreateAt)) {
				running = item
			}
			continue
		}
		blocked = true
	}
	if running != nil {
		return running, false, nil
	}
	if blocked {
		return nil, true, nil
	}
	defaults, err := s.employees.List(ctx,
		dataaccess.Eq("tenant_id", tenantID),
		dataaccess.Eq("is_default", true),
		dataaccess.Eq("status", employeeStatusRunning),
		dataaccess.IsNull("deleted_at"),
		dataaccess.Order("create_at", dataaccess.OrderTypeAsc),
		dataaccess.Order("id", dataaccess.OrderTypeAsc),
	)
	if err != nil {
		return nil, false, err
	}
	if len(defaults) == 0 || defaults[0] == nil {
		return nil, false, nil
	}
	return defaults[0], false, nil
}

func (s *RelayService) backfillEmployeeTenants(ctx context.Context) error {
	users, err := s.userTenants.List(ctx)
	if err != nil {
		if isMissingTable(err) {
			return nil
		}
		return err
	}
	byUser := make(map[string]string, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		byUser[user.UserKey] = strings.TrimSpace(user.TenantID)
	}
	employees, err := s.employees.List(ctx)
	if err != nil {
		if isMissingTable(err) {
			return nil
		}
		return err
	}
	byEmployee := make(map[string]string, len(employees))
	for _, item := range employees {
		if item == nil {
			continue
		}
		if strings.TrimSpace(item.TenantID) == "" {
			item.TenantID = byUser[item.AccountKey]
			if item.TenantID == "" {
				continue
			}
			if err := s.employees.Update(ctx, item); err != nil {
				return err
			}
		}
		byEmployee[item.EmployeeKey] = item.TenantID
	}
	bindings, err := s.bindings.List(ctx)
	if err != nil {
		if isMissingTable(err) {
			return nil
		}
		return err
	}
	for _, binding := range bindings {
		if binding == nil || strings.TrimSpace(binding.TenantID) != "" {
			continue
		}
		binding.TenantID = byEmployee[binding.EmployeeKey]
		if binding.TenantID == "" {
			continue
		}
		if err := s.bindings.Update(ctx, binding); err != nil {
			return err
		}
	}
	return nil
}

func isMissingTable(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no such table") || strings.Contains(message, "doesn't exist") || strings.Contains(message, "does not exist")
}
