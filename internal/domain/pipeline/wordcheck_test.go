package pipeline

import (
	"testing"
)

func TestCheckOutlineWordCount(t *testing.T) {
	t.Run("skip check", func(t *testing.T) {
		r := CheckOutlineWordCount("some outline", 3000, true)
		if r != nil {
			t.Error("should return nil when skipped")
		}
	})
	t.Run("target zero", func(t *testing.T) {
		r := CheckOutlineWordCount("outline", 0, false)
		if r != nil {
			t.Error("should return nil when target is 0")
		}
	})
	t.Run("empty", func(t *testing.T) {
		r := CheckOutlineWordCount("", 3000, false)
		if r != nil {
			t.Error("should return nil for empty outline")
		}
	})
	t.Run("within range", func(t *testing.T) {
		outline := "全文建议字数汇总：3200"
		r := CheckOutlineWordCount(outline, 3000, false)
		if r != nil {
			t.Errorf("within 30%%: got %v, want nil", r)
		}
	})
	t.Run("exceeds range", func(t *testing.T) {
		outline := "全文建议字数汇总：5000"
		r := CheckOutlineWordCount(outline, 3000, false)
		if r == nil || !r.Mismatch {
			t.Error("should detect mismatch when 5000 vs 3000")
		}
	})
	t.Run("too low", func(t *testing.T) {
		outline := "全文建议字数汇总：1500"
		r := CheckOutlineWordCount(outline, 3000, false)
		if r == nil || !r.Mismatch {
			t.Error("should detect mismatch when 1500 vs 3000")
		}
	})
	t.Run("node estimates summed", func(t *testing.T) {
		outline := "预估字数：500\n另一个预估字数：600\n第三个预估字数：700"
		r := CheckOutlineWordCount(outline, 1500, false)
		if r != nil {
			t.Errorf("summed 1800 vs 1500 within range: got %v", r)
		}
	})
}
