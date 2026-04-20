package products

import (
	"math"
	"strings"
)

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func parsePricingType(v any) (PricingType, bool) {
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	switch PricingType(s) {
	case PricingFixed, PricingOnRequest:
		return PricingType(s), true
	}
	return "", false
}

func parseRequiredString(v any) (string, bool) {
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	t := strings.TrimSpace(s)
	if t == "" {
		return "", false
	}
	return t, true
}

// parseOptionalString returns (value, present, valid).
// - absent: present=false, valid=true
// - null:   present=true, value=nil, valid=true
// - empty trimmed string: present=true, value=nil, valid=true
// - wrong type: present=true, value=nil, valid=false
func parseOptionalString(m map[string]any, key string) (*string, bool, bool) {
	v, ok := m[key]
	if !ok {
		return nil, false, true
	}
	if v == nil {
		return nil, true, true
	}
	s, isStr := v.(string)
	if !isStr {
		return nil, true, false
	}
	t := strings.TrimSpace(s)
	if t == "" {
		return nil, true, true
	}
	return &t, true, true
}

func parseOptionalMinor(m map[string]any, key string) (*int64, bool, bool) {
	v, ok := m[key]
	if !ok {
		return nil, false, true
	}
	if v == nil {
		return nil, true, true
	}
	f, ok := v.(float64)
	if !ok || math.IsNaN(f) || math.IsInf(f, 0) || f < 0 {
		return nil, true, false
	}
	n := int64(math.Round(f))
	return &n, true, true
}

func parseOptionalSortOrder(v any) (*int, bool) {
	f, ok := v.(float64)
	if !ok || math.IsNaN(f) || math.IsInf(f, 0) {
		return nil, false
	}
	n := int(math.Round(f))
	return &n, true
}

func ParseCreateInput(body any) *CreateInput {
	m := asMap(body)
	if m == nil {
		return nil
	}

	slug, ok := parseRequiredString(m["slug"])
	if !ok {
		return nil
	}
	nameUk, ok := parseRequiredString(m["nameUk"])
	if !ok {
		return nil
	}
	nameEn, ok := parseRequiredString(m["nameEn"])
	if !ok {
		return nil
	}
	pricing, ok := parsePricingType(m["pricingType"])
	if !ok {
		return nil
	}

	priceUah, presentUah, validUah := parseOptionalMinor(m, "priceUahMinor")
	if !validUah {
		return nil
	}
	priceUsd, presentUsd, validUsd := parseOptionalMinor(m, "priceUsdMinor")
	if !validUsd {
		return nil
	}

	if pricing == PricingFixed {
		if !presentUah || !presentUsd || priceUah == nil || priceUsd == nil {
			return nil
		}
	}

	descUk, _, validDescUk := parseOptionalString(m, "descriptionUk")
	if !validDescUk {
		return nil
	}
	descEn, _, validDescEn := parseOptionalString(m, "descriptionEn")
	if !validDescEn {
		return nil
	}
	imageURL, _, validImage := parseOptionalString(m, "imageUrl")
	if !validImage {
		return nil
	}

	active := true
	if v, ok := m["active"].(bool); ok {
		active = v
	}

	sortOrder := 0
	if v, present := m["sortOrder"]; present {
		if parsed, ok := parseOptionalSortOrder(v); ok {
			sortOrder = *parsed
		} else {
			return nil
		}
	}

	return &CreateInput{
		Slug:          slug,
		NameUk:        nameUk,
		NameEn:        nameEn,
		DescriptionUk: descUk,
		DescriptionEn: descEn,
		PricingType:   pricing,
		PriceUahMinor: priceUah,
		PriceUsdMinor: priceUsd,
		ImageURL:      imageURL,
		Active:        active,
		SortOrder:     sortOrder,
	}
}

func ParseUpdateInput(body any) *UpdateInput {
	m := asMap(body)
	if m == nil {
		return nil
	}
	update := &UpdateInput{}

	if raw, ok := m["slug"]; ok {
		s, valid := parseRequiredString(raw)
		if !valid {
			return nil
		}
		update.Slug = &s
	}
	if raw, ok := m["nameUk"]; ok {
		s, valid := parseRequiredString(raw)
		if !valid {
			return nil
		}
		update.NameUk = &s
	}
	if raw, ok := m["nameEn"]; ok {
		s, valid := parseRequiredString(raw)
		if !valid {
			return nil
		}
		update.NameEn = &s
	}

	if descUk, present, valid := parseOptionalString(m, "descriptionUk"); present {
		if !valid {
			return nil
		}
		update.DescriptionUk = descUk
		update.HasDescUk = true
	}
	if descEn, present, valid := parseOptionalString(m, "descriptionEn"); present {
		if !valid {
			return nil
		}
		update.DescriptionEn = descEn
		update.HasDescEn = true
	}

	if raw, ok := m["pricingType"]; ok {
		pt, valid := parsePricingType(raw)
		if !valid {
			return nil
		}
		update.PricingType = &pt
	}

	if price, present, valid := parseOptionalMinor(m, "priceUahMinor"); present {
		if !valid {
			return nil
		}
		update.PriceUahMinor = price
		update.HasPriceUah = true
	}
	if price, present, valid := parseOptionalMinor(m, "priceUsdMinor"); present {
		if !valid {
			return nil
		}
		update.PriceUsdMinor = price
		update.HasPriceUsd = true
	}

	if img, present, valid := parseOptionalString(m, "imageUrl"); present {
		if !valid {
			return nil
		}
		update.ImageURL = img
		update.HasImageURL = true
	}

	if raw, ok := m["active"]; ok {
		b, isBool := raw.(bool)
		if !isBool {
			return nil
		}
		update.Active = &b
	}

	if raw, ok := m["sortOrder"]; ok {
		n, valid := parseOptionalSortOrder(raw)
		if !valid {
			return nil
		}
		update.SortOrder = n
	}

	return update
}
