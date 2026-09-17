package service

import (
	"errors"
	"testing"

	"github.com/dezhishen/parking-violation-assistant/internal/models"
)

func TestValidateStatusTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		wantErr bool
	}{
		{"待处理 -> 待确认", models.StatusPending, models.StatusWaitConfirm, false},
		{"待确认 -> 违停", models.StatusWaitConfirm, models.StatusViolation, false},
		{"待确认 -> 已挪车", models.StatusWaitConfirm, models.StatusMoved, false},
		{"同状态幂等", models.StatusViolation, models.StatusViolation, false},
		{"待处理 -> 违停 非法", models.StatusPending, models.StatusViolation, true},
		{"违停 为终态", models.StatusViolation, models.StatusMoved, true},
		{"已挪车 为终态", models.StatusMoved, models.StatusViolation, true},
		{"未知状态", "未知", models.StatusViolation, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStatusTransition(tt.from, tt.to)
			if tt.wantErr && err == nil {
				t.Fatalf("期望报错，实际通过")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("期望通过，实际报错: %v", err)
			}
			if tt.wantErr && !errors.Is(err, ErrInvalidTransition) {
				t.Fatalf("期望 ErrInvalidTransition，实际: %v", err)
			}
		})
	}
}

func TestBuildWhere(t *testing.T) {
	tests := []struct {
		name      string
		filters   models.QueryFilters
		wantSQL   string
		wantArgs  []any
		wantEmpty bool
	}{
		{
			name:      "无条件",
			filters:   models.QueryFilters{},
			wantEmpty: true,
		},
		{
			name:     "车牌模糊匹配",
			filters:  models.QueryFilters{PlateKeyword: "粤A"},
			wantSQL:  " WHERE plate_number LIKE ?",
			wantArgs: []any{"%粤A%"},
		},
		{
			name:     "指定状态",
			filters:  models.QueryFilters{Status: models.StatusViolation},
			wantSQL:  " WHERE status = ?",
			wantArgs: []any{models.StatusViolation},
		},
		{
			name:      "status=all 不筛选状态",
			filters:   models.QueryFilters{Status: models.StatusAll},
			wantEmpty: true,
		},
		{
			name:     "结束日期用次日零点闭合区间",
			filters:  models.QueryFilters{EndDate: "2026-09-30"},
			wantSQL:  " WHERE created_at < ?",
			wantArgs: []any{"2026-10-01 00:00:00"},
		},
		{
			name:     "非法结束日期退回字符串比较",
			filters:  models.QueryFilters{EndDate: "不是日期"},
			wantSQL:  " WHERE created_at <= ?",
			wantArgs: []any{"不是日期 23:59:59"},
		},
		{
			name: "组合条件",
			filters: models.QueryFilters{
				PlateKeyword: "粤A",
				Status:       models.StatusMoved,
				StartDate:    "2026-09-01",
				EndDate:      "2026-09-30",
			},
			wantSQL:  " WHERE plate_number LIKE ? AND status = ? AND created_at >= ? AND created_at < ?",
			wantArgs: []any{"%粤A%", models.StatusMoved, "2026-09-01 00:00:00", "2026-10-01 00:00:00"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args := buildWhere(tt.filters)
			if tt.wantEmpty {
				if sql != "" || len(args) != 0 {
					t.Fatalf("期望空条件，实际 sql=%q args=%v", sql, args)
				}
				return
			}
			if sql != tt.wantSQL {
				t.Fatalf("SQL 不匹配\n期望: %q\n实际: %q", tt.wantSQL, sql)
			}
			if len(args) != len(tt.wantArgs) {
				t.Fatalf("参数数量不匹配，期望 %v 实际 %v", tt.wantArgs, args)
			}
			for i := range args {
				if args[i] != tt.wantArgs[i] {
					t.Fatalf("第 %d 个参数不匹配，期望 %v 实际 %v", i, tt.wantArgs[i], args[i])
				}
			}
		})
	}
}

func TestWarningThreshold(t *testing.T) {
	tests := []struct {
		name    string
		filters models.QueryFilters
		want    int
	}{
		{"未设置时用默认值", models.QueryFilters{}, models.DefaultWarningThreshold},
		{"负数用默认值", models.QueryFilters{WarningThreshold: -1}, models.DefaultWarningThreshold},
		{"使用自定义阈值", models.QueryFilters{WarningThreshold: 5}, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := warningThreshold(tt.filters); got != tt.want {
				t.Fatalf("期望 %d，实际 %d", tt.want, got)
			}
		})
	}
}

func TestHavingClause(t *testing.T) {
	if got := havingClause(models.QueryFilters{}, 3); got != "" {
		t.Fatalf("未勾选时不应生成 HAVING，实际 %q", got)
	}
	got := havingClause(models.QueryFilters{OverThreeWarning: true}, 5)
	if got != " HAVING COUNT(*) >= 5" {
		t.Fatalf("期望 HAVING COUNT(*) >= 5，实际 %q", got)
	}
}

func TestNextDay(t *testing.T) {
	tests := []struct {
		date string
		want string
		ok   bool
	}{
		{"2026-09-30", "2026-10-01", true},
		{"2026-12-31", "2027-01-01", true},
		{"bad", "", false},
	}

	for _, tt := range tests {
		got, ok := nextDay(tt.date)
		if ok != tt.ok || got != tt.want {
			t.Fatalf("nextDay(%q) = (%q, %v)，期望 (%q, %v)", tt.date, got, ok, tt.want, tt.ok)
		}
	}
}
