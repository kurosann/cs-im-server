package service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"vane-common/pkg/dataaccess"
	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/idutil"
)

const (
	employeeStatusRunning     = "running"
	employeeStatusPaused      = "paused"
	employeeModeBasic         = "basic"
	employeeModeProfessional  = "professional"
	employeeHistoryDefault    = 10
	employeeHistoryMin        = 1
	employeeHistoryMax        = 50
	employeeNameMaxRunes      = 50
	employeeDeletedNamePrefix = "__deleted__:"
)

type AccountBinding struct {
	Platform   string `json:"platform"`
	AccountKey string `json:"accountKey"`
	Label      string `json:"label,omitempty"`
}

type EmployeeView struct {
	EmployeeID      string           `json:"employeeId"`
	Name            string           `json:"name"`
	Persona         string           `json:"persona"`
	Status          string           `json:"status"`
	Mode            string           `json:"mode"`
	Prompt          string           `json:"prompt"`
	History         int              `json:"history"`
	AutoReply       bool             `json:"autoReply"`
	SelectedDocIDs  []string         `json:"selectedDocIds"`
	Tools           []string         `json:"tools"`
	AccountBindings []AccountBinding `json:"accountBindings"`
}

type CreateEmployeeRequest struct {
	Name string `json:"name"`
}

type UpdateEmployeeRequest struct {
	Name            string           `json:"name"`
	Persona         string           `json:"persona"`
	Status          string           `json:"status"`
	Mode            string           `json:"mode"`
	Prompt          string           `json:"prompt"`
	History         int              `json:"history"`
	AutoReply       bool             `json:"autoReply"`
	SelectedDocIDs  []string         `json:"selectedDocIds"`
	Tools           []string         `json:"tools"`
	AccountBindings []AccountBinding `json:"accountBindings"`
}

func (s *RelayService) ListEmployees(ctx context.Context, accountKey, tenantID string) ([]EmployeeView, error) {
	accountKey = strings.TrimSpace(accountKey)
	if accountKey == "" {
		return nil, unauthorized("请先登录")
	}
	items, err := s.employees.List(ctx,
		dataaccess.Eq("account_key", accountKey),
		dataaccess.IsNull("deleted_at"),
		dataaccess.Order("create_at", dataaccess.OrderTypeAsc),
		dataaccess.Order("id", dataaccess.OrderTypeAsc),
	)
	if err != nil {
		return nil, err
	}
	labels, err := s.accountLabels(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	views := make([]EmployeeView, 0, len(items))
	for _, item := range items {
		bindings, err := s.bindingsFor(ctx, accountKey, item.EmployeeKey, labels)
		if err != nil {
			return nil, err
		}
		views = append(views, employeeView(item, bindings))
	}
	return views, nil
}

func (s *RelayService) ListEmployeeAccounts(ctx context.Context, tenantID string) ([]AccountBinding, error) {
	return s.accountOptions(ctx, strings.TrimSpace(tenantID))
}

func (s *RelayService) CreateEmployee(ctx context.Context, accountKey string, req CreateEmployeeRequest) (EmployeeView, error) {
	accountKey = strings.TrimSpace(accountKey)
	if accountKey == "" {
		return EmployeeView{}, unauthorized("请先登录")
	}
	name, nameKey, err := normalizeEmployeeName(req.Name)
	if err != nil {
		return EmployeeView{}, err
	}
	if err := s.ensureEmployeeNameAvailable(ctx, accountKey, nameKey, ""); err != nil {
		return EmployeeView{}, err
	}
	item := &AIEmployee{
		EmployeeKey:         "emp_" + idutil.GenShortId(),
		AccountKey:          accountKey,
		Name:                name,
		NameKey:             nameKey,
		Mode:                employeeModeBasic,
		HistoryMessageCount: employeeHistoryDefault,
		AutoReplyEnabled:    true,
		Status:              employeeStatusPaused,
		SelectedDocIDs:      []string{},
		Tools:               []string{},
	}
	if err := s.employees.Create(ctx, item); err != nil {
		if isDuplicateKey(err) {
			return EmployeeView{}, conflict("名称已存在")
		}
		return EmployeeView{}, err
	}
	return employeeView(item, []AccountBinding{}), nil
}

func (s *RelayService) UpdateEmployee(ctx context.Context, accountKey, tenantID, employeeID string, req UpdateEmployeeRequest) (EmployeeView, error) {
	accountKey = strings.TrimSpace(accountKey)
	employeeID = strings.TrimSpace(employeeID)
	if accountKey == "" {
		return EmployeeView{}, unauthorized("请先登录")
	}
	name, nameKey, err := normalizeEmployeeName(req.Name)
	if err != nil {
		return EmployeeView{}, err
	}
	mode, err := normalizeEmployeeMode(req.Mode)
	if err != nil {
		return EmployeeView{}, err
	}
	status, err := normalizeEmployeeStatus(req.Status)
	if err != nil {
		return EmployeeView{}, err
	}
	if req.History < employeeHistoryMin || req.History > employeeHistoryMax {
		return EmployeeView{}, badRequest("上下文条数需要在 1 到 50 之间")
	}
	persona := strings.TrimSpace(req.Persona)
	if utf8.RuneCountInString(persona) > 255 {
		return EmployeeView{}, badRequest("人设不能超过 255 个字")
	}
	docs := normalizeStringList(req.SelectedDocIDs)
	tools := normalizeStringList(req.Tools)
	options, err := s.accountOptions(ctx, tenantID)
	if err != nil {
		return EmployeeView{}, err
	}
	desired, err := normalizeBindings(req.AccountBindings, options)
	if err != nil {
		return EmployeeView{}, err
	}

	var saved *AIEmployee
	err = s.employees.Transaction(ctx, func(txCtx context.Context) error {
		item, err := s.findEmployee(txCtx, accountKey, employeeID)
		if err != nil {
			return err
		}
		if err := s.ensureEmployeeNameAvailable(txCtx, accountKey, nameKey, employeeID); err != nil {
			return err
		}
		if err := s.ensureBindingsAvailable(txCtx, accountKey, employeeID, desired); err != nil {
			return err
		}
		if err := s.replaceBindings(txCtx, accountKey, employeeID, desired); err != nil {
			return err
		}
		item.Name = name
		item.NameKey = nameKey
		item.Persona = persona
		item.Mode = mode
		item.Prompt = req.Prompt
		item.HistoryMessageCount = req.History
		item.AutoReplyEnabled = req.AutoReply
		item.Status = status
		item.SelectedDocIDs = docs
		item.Tools = tools
		if err := s.employees.Update(txCtx, item); err != nil {
			if isDuplicateKey(err) {
				return conflict("名称已存在")
			}
			return err
		}
		saved = item
		return nil
	})
	if err != nil {
		return EmployeeView{}, err
	}
	labels := labelIndex(options)
	return employeeView(saved, bindingsWithLabels(desired, labels)), nil
}

func (s *RelayService) DeleteEmployee(ctx context.Context, accountKey, employeeID string) error {
	accountKey = strings.TrimSpace(accountKey)
	employeeID = strings.TrimSpace(employeeID)
	if accountKey == "" {
		return unauthorized("请先登录")
	}
	return s.employees.Transaction(ctx, func(txCtx context.Context) error {
		item, err := s.findEmployee(txCtx, accountKey, employeeID)
		if err != nil {
			return err
		}
		if err := s.replaceBindings(txCtx, accountKey, employeeID, nil); err != nil {
			return err
		}
		now := time.Now()
		item.DeletedAt = &now
		item.NameKey = employeeDeletedNamePrefix + item.EmployeeKey
		item.SelectedDocIDs = []string{}
		item.Tools = []string{}
		return s.employees.Update(txCtx, item)
	})
}

func (s *RelayService) findEmployee(ctx context.Context, accountKey, employeeID string) (*AIEmployee, error) {
	item, err := s.employees.First(ctx,
		dataaccess.Eq("account_key", accountKey),
		dataaccess.Eq("employee_key", employeeID),
		dataaccess.IsNull("deleted_at"),
	)
	if err != nil {
		if errors.Is(err, dataaccess.ErrDataNotFound) {
			return nil, notFound("AI 客服员工不存在")
		}
		return nil, err
	}
	return item, nil
}

func (s *RelayService) ensureEmployeeNameAvailable(ctx context.Context, accountKey, nameKey, exceptID string) error {
	item, err := s.employees.First(ctx,
		dataaccess.Eq("account_key", accountKey),
		dataaccess.Eq("name_key", nameKey),
		dataaccess.IsNull("deleted_at"),
	)
	if errors.Is(err, dataaccess.ErrDataNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if item.EmployeeKey != exceptID {
		return conflict("名称已存在")
	}
	return nil
}

func (s *RelayService) ensureBindingsAvailable(ctx context.Context, accountKey, employeeID string, desired []AccountBinding) error {
	for _, binding := range desired {
		current, err := s.bindings.First(ctx,
			dataaccess.Eq("account_key", accountKey),
			dataaccess.Eq("platform", binding.Platform),
			dataaccess.Eq("account_key_binding", binding.AccountKey),
		)
		if errors.Is(err, dataaccess.ErrDataNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if current.EmployeeKey != employeeID {
			return conflict("账号 %s 已绑定到其他 AI 客服员工", binding.LabelOrKey())
		}
	}
	return nil
}

func (s *RelayService) replaceBindings(ctx context.Context, accountKey, employeeID string, desired []AccountBinding) error {
	current, err := s.bindings.List(ctx,
		dataaccess.Eq("account_key", accountKey),
		dataaccess.Eq("employee_key", employeeID),
	)
	if err != nil {
		return err
	}
	for _, binding := range current {
		if err := s.bindings.Delete(ctx, binding); err != nil {
			return err
		}
	}
	for _, binding := range desired {
		row := &AIEmployeeBinding{
			AccountKey:      accountKey,
			EmployeeKey:     employeeID,
			Platform:        binding.Platform,
			BoundAccountKey: binding.AccountKey,
		}
		if err := s.bindings.Create(ctx, row); err != nil {
			if isDuplicateKey(err) {
				return conflict("账号 %s 已绑定到其他 AI 客服员工", binding.LabelOrKey())
			}
			return err
		}
	}
	return nil
}

func (s *RelayService) bindingsFor(ctx context.Context, accountKey, employeeID string, labels map[string]string) ([]AccountBinding, error) {
	rows, err := s.bindings.List(ctx,
		dataaccess.Eq("account_key", accountKey),
		dataaccess.Eq("employee_key", employeeID),
		dataaccess.Order("platform", dataaccess.OrderTypeAsc),
		dataaccess.Order("account_key_binding", dataaccess.OrderTypeAsc),
	)
	if err != nil {
		return nil, err
	}
	out := make([]AccountBinding, 0, len(rows))
	for _, row := range rows {
		binding := AccountBinding{Platform: row.Platform, AccountKey: row.BoundAccountKey}
		binding.Label = labels[binding.identity()]
		if binding.Label == "" {
			binding.Label = binding.AccountKey
		}
		out = append(out, binding)
	}
	return out, nil
}

func (s *RelayService) accountLabels(ctx context.Context, tenantID string) (map[string]string, error) {
	options, err := s.accountOptions(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return labelIndex(options), nil
}

func (s *RelayService) accountOptions(ctx context.Context, tenantID string) ([]AccountBinding, error) {
	channels, err := s.channelAccessor.List(ctx)
	if err != nil {
		return nil, err
	}
	var apps []*businessAppRow
	if tenantID != "" {
		apps, err = s.apps.List(ctx, dataaccess.Eq("tenant_id", tenantID))
		if err != nil {
			return nil, err
		}
	}
	options := make([]AccountBinding, 0, len(channels)+len(apps))
	for _, app := range apps {
		if app == nil || strings.TrimSpace(app.AppKey) == "" {
			continue
		}
		platform := normalizePlatform(app.Kind)
		if platform == "" {
			platform = "app"
		}
		label := strings.TrimSpace(app.DisplayName)
		if label == "" {
			label = app.AppKey
		}
		options = append(options, AccountBinding{Platform: platform, AccountKey: app.AppKey, Label: label})
	}
	for _, channel := range channels {
		if channel == nil || strings.TrimSpace(channel.ChannelKey) == "" {
			continue
		}
		platform := normalizePlatform(channel.Type)
		if platform == "" {
			platform = "channel"
		}
		label := strings.TrimSpace(channel.Name)
		if label == "" {
			label = channel.ChannelKey
		}
		options = append(options, AccountBinding{Platform: platform, AccountKey: channel.ChannelKey, Label: label})
	}
	sort.Slice(options, func(i, j int) bool {
		if options[i].Platform == options[j].Platform {
			return options[i].Label < options[j].Label
		}
		return options[i].Platform < options[j].Platform
	})
	return options, nil
}

func employeeView(item *AIEmployee, bindings []AccountBinding) EmployeeView {
	if bindings == nil {
		bindings = []AccountBinding{}
	}
	return EmployeeView{
		EmployeeID:      item.EmployeeKey,
		Name:            item.Name,
		Persona:         item.Persona,
		Status:          item.Status,
		Mode:            item.Mode,
		Prompt:          item.Prompt,
		History:         item.HistoryMessageCount,
		AutoReply:       item.AutoReplyEnabled,
		SelectedDocIDs:  stringList(item.SelectedDocIDs),
		Tools:           stringList(item.Tools),
		AccountBindings: bindings,
	}
}

func normalizeEmployeeName(value string) (string, string, error) {
	name := strings.TrimSpace(value)
	if name == "" {
		return "", "", badRequest("名称不能为空")
	}
	if utf8.RuneCountInString(name) > employeeNameMaxRunes {
		return "", "", badRequest("名称不能超过 50 个字")
	}
	return name, strings.ToLower(name), nil
}

func normalizeEmployeeMode(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case employeeModeBasic, employeeModeProfessional:
		return strings.TrimSpace(value), nil
	default:
		return "", badRequest("不支持的配置模式")
	}
}

func normalizeEmployeeStatus(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case employeeStatusRunning, employeeStatusPaused:
		return strings.TrimSpace(value), nil
	default:
		return "", badRequest("不支持的运行状态")
	}
}

func normalizeStringList(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func normalizeBindings(values, options []AccountBinding) ([]AccountBinding, error) {
	allowed := make(map[string]AccountBinding, len(options))
	for _, option := range options {
		allowed[option.identity()] = option
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]AccountBinding, 0, len(values))
	for _, value := range values {
		value.Platform = normalizePlatform(value.Platform)
		value.AccountKey = strings.TrimSpace(value.AccountKey)
		if value.Platform == "" || value.AccountKey == "" {
			return nil, badRequest("账号绑定不完整")
		}
		option, ok := allowed[value.identity()]
		if !ok {
			return nil, badRequest("账号不存在")
		}
		if _, ok := seen[value.identity()]; ok {
			continue
		}
		seen[value.identity()] = struct{}{}
		out = append(out, option)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Platform == out[j].Platform {
			return out[i].AccountKey < out[j].AccountKey
		}
		return out[i].Platform < out[j].Platform
	})
	return out, nil
}

func normalizePlatform(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func (b AccountBinding) identity() string {
	return normalizePlatform(b.Platform) + "\x00" + strings.TrimSpace(b.AccountKey)
}

func (b AccountBinding) LabelOrKey() string {
	if label := strings.TrimSpace(b.Label); label != "" {
		return label
	}
	return b.AccountKey
}

func labelIndex(options []AccountBinding) map[string]string {
	labels := make(map[string]string, len(options))
	for _, option := range options {
		labels[option.identity()] = option.Label
	}
	return labels
}

func bindingsWithLabels(bindings []AccountBinding, labels map[string]string) []AccountBinding {
	out := make([]AccountBinding, 0, len(bindings))
	for _, binding := range bindings {
		if binding.Label == "" {
			binding.Label = labels[binding.identity()]
		}
		if binding.Label == "" {
			binding.Label = binding.AccountKey
		}
		out = append(out, binding)
	}
	return out
}

func stringList(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return cloneStrings(values)
}

// businessAppRow reads the gateway apps table without importing that module.
type businessAppRow struct {
	base.BaseModel
	AppKey      string `gorm:"column:app_key"`
	TenantID    string `gorm:"column:tenant_id"`
	DisplayName string `gorm:"column:display_name"`
	Kind        string `gorm:"column:kind"`
}

func (*businessAppRow) TableName() string { return "apps" }

func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "duplicate")
}
