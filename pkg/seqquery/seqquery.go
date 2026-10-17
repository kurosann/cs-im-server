package seqquery

import (
	"strconv"

	"cs-im-server/pkg/apperr"
	"cs-im-server/pkg/httpapi"
)

type Filter struct {
	AfterSeq  int64
	BeforeSeq int64
	Limit     int
	Order     string
}

func FromHTTP(ctx httpapi.Context) (Filter, error) {
	filter := Filter{}
	if err := parseInt(ctx.Query("afterSeq"), &filter.AfterSeq, "afterSeq"); err != nil {
		return Filter{}, err
	}
	if err := parseInt(ctx.Query("beforeSeq"), &filter.BeforeSeq, "beforeSeq"); err != nil {
		return Filter{}, err
	}
	if raw := ctx.Query("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			return Filter{}, apperr.BadRequest("limit must be a non-negative integer")
		}
		filter.Limit = value
	}
	filter.Order = ctx.Query("order")
	return filter, nil
}

func parseInt(raw string, target *int64, name string) error {
	if raw == "" {
		return nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return apperr.BadRequest("%s must be a non-negative integer", name)
	}
	*target = value
	return nil
}
