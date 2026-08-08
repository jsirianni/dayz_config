package weatherconfig

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type ValidationErrors []string

func (errs ValidationErrors) Error() string {
	return strings.Join(errs, "; ")
}

type fieldKind int

const (
	fieldBool fieldKind = iota
	fieldFloat
)

type fieldRule struct {
	Kind fieldKind
	Min  *float64
	Max  *float64
}

type elementSpec struct {
	Fields   map[string]fieldRule
	Children map[string]elementSpec
}

var (
	readFile = os.ReadFile

	zero = 0.0
	one  = 1.0

	anyFloat = fieldRule{Kind: fieldFloat}
	zeroOne  = fieldRule{Kind: fieldFloat, Min: &zero, Max: &one}
	boolRule = fieldRule{Kind: fieldBool}

	rootSpec = elementSpec{
		Fields: map[string]fieldRule{
			"reset":  boolRule,
			"enable": boolRule,
		},
		Children: map[string]elementSpec{
			"overcast":      weatherValueSpec(true, false),
			"fog":           weatherValueSpec(true, false),
			"rain":          weatherValueSpec(true, true),
			"windMagnitude": weatherValueSpec(false, false),
			"windDirection": weatherValueSpec(false, false),
			"snowfall":      weatherValueSpec(true, true),
			"storm": {
				Fields: map[string]fieldRule{
					"density":   zeroOne,
					"threshold": zeroOne,
					"timeout":   anyFloat,
				},
			},
			"wind": {
				Fields: map[string]fieldRule{
					"maxspeed": anyFloat,
				},
				Children: map[string]elementSpec{
					"params": {
						Fields: map[string]fieldRule{
							"min":       zeroOne,
							"max":       zeroOne,
							"frequency": anyFloat,
						},
					},
				},
			},
		},
	}
)

type validator struct {
	decoder *xml.Decoder
	errs    ValidationErrors
}

func ValidateFile(path string) error {
	data, err := readFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return ValidateData(data, path)
}

func ValidateData(data []byte, sourceName string) error {
	errs, err := validateXML(data)
	if err != nil {
		return fmt.Errorf("parse %s: %w", sourceName, err)
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

func validateXML(data []byte) (ValidationErrors, error) {
	v := validator{decoder: xml.NewDecoder(bytes.NewReader(data))}
	seenRoot := false
	for {
		token, err := v.decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		switch value := token.(type) {
		case xml.StartElement:
			if seenRoot {
				v.add("multiple root elements")
				if err := v.skipElement(value); err != nil {
					return nil, err
				}
				continue
			}
			seenRoot = true
			if value.Name.Local != "weather" {
				v.add("expected <weather> root, got <%s>", value.Name.Local)
				if err := v.skipElement(value); err != nil {
					return nil, err
				}
				continue
			}
			if err := v.validateElement(value, rootSpec, "weather"); err != nil {
				return nil, err
			}
		case xml.CharData:
			if strings.TrimSpace(string(value)) != "" {
				v.add("unexpected text outside root")
			}
		}
	}
	if !seenRoot {
		v.add("missing <weather> root")
	}
	return v.errs, nil
}

func weatherValueSpec(intensity bool, thresholds bool) elementSpec {
	valueRule := anyFloat
	if intensity {
		valueRule = zeroOne
	}
	children := map[string]elementSpec{
		"current": {
			Fields: map[string]fieldRule{
				"actual":   valueRule,
				"time":     anyFloat,
				"duration": anyFloat,
			},
		},
		"limits":       minMaxSpec(valueRule),
		"timelimits":   minMaxSpec(anyFloat),
		"changelimits": minMaxSpec(valueRule),
	}
	if thresholds {
		children["thresholds"] = elementSpec{Fields: map[string]fieldRule{
			"min": zeroOne,
			"max": zeroOne,
			"end": anyFloat,
		}}
	}
	return elementSpec{Children: children}
}

func minMaxSpec(rule fieldRule) elementSpec {
	return elementSpec{Fields: map[string]fieldRule{"min": rule, "max": rule}}
}

func (v *validator) validateElement(start xml.StartElement, spec elementSpec, path string) error {
	seen := map[string]string{}
	numbers := map[string]float64{}
	v.validateAttributes(start, spec, path, seen, numbers)
	for {
		token, err := v.decoder.Token()
		if err != nil {
			return err
		}
		switch value := token.(type) {
		case xml.StartElement:
			name := value.Name.Local
			if child, ok := spec.Children[name]; ok {
				if err := v.validateElement(value, child, path+"."+name); err != nil {
					return err
				}
				continue
			}
			if rule, ok := spec.Fields[name]; ok {
				if err := v.validateScalarElement(value, path, name, rule, seen, numbers); err != nil {
					return err
				}
				continue
			}
			v.add("unknown element <%s.%s>", path, name)
			if err := v.skipElement(value); err != nil {
				return err
			}
		case xml.CharData:
			if strings.TrimSpace(string(value)) != "" {
				v.add("unexpected text in <%s>", path)
			}
		case xml.EndElement:
			v.validateMinMax(path, numbers)
			return nil
		}
	}
}

func (v *validator) validateAttributes(start xml.StartElement, spec elementSpec, path string, seen map[string]string, numbers map[string]float64) {
	for _, attr := range start.Attr {
		name := attr.Name.Local
		rule, ok := spec.Fields[name]
		if !ok {
			v.add("unknown attribute %s@%s", path, name)
			continue
		}
		v.validateField(path, name, attr.Value, "attribute", rule, seen, numbers)
	}
}

func (v *validator) validateScalarElement(start xml.StartElement, parentPath string, field string, rule fieldRule, seen map[string]string, numbers map[string]float64) error {
	path := parentPath + "." + field
	for _, attr := range start.Attr {
		v.add("unknown attribute %s@%s", path, attr.Name.Local)
	}
	var text strings.Builder
	for {
		token, err := v.decoder.Token()
		if err != nil {
			return err
		}
		switch value := token.(type) {
		case xml.StartElement:
			v.add("unexpected element <%s.%s>", path, value.Name.Local)
			if err := v.skipElement(value); err != nil {
				return err
			}
		case xml.CharData:
			text.WriteString(string(value))
		case xml.EndElement:
			v.validateField(parentPath, field, strings.TrimSpace(text.String()), "element", rule, seen, numbers)
			return nil
		}
	}
}

func (v *validator) validateField(path string, field string, raw string, source string, rule fieldRule, seen map[string]string, numbers map[string]float64) {
	if previous, ok := seen[field]; ok {
		v.add("duplicate field %s.%s as %s and %s", path, field, previous, source)
		return
	}
	seen[field] = source
	switch rule.Kind {
	case fieldBool:
		if !isBool(raw) {
			v.add("%s.%s must be a boolean", path, field)
		}
	case fieldFloat:
		number, ok := v.parseFloat(path, field, raw)
		if !ok {
			return
		}
		numbers[field] = number
		if rule.Min != nil && number < *rule.Min {
			v.add("%s.%s must be greater than or equal to %s", path, field, formatFloat(*rule.Min))
		}
		if rule.Max != nil && number > *rule.Max {
			v.add("%s.%s must be less than or equal to %s", path, field, formatFloat(*rule.Max))
		}
	}
}

func (v *validator) parseFloat(path string, field string, raw string) (float64, bool) {
	number, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		v.add("%s.%s must be a number", path, field)
		return 0, false
	}
	return number, true
}

func (v *validator) validateMinMax(path string, numbers map[string]float64) {
	min, hasMin := numbers["min"]
	max, hasMax := numbers["max"]
	if hasMin && hasMax && min > max {
		v.add("%s min must be less than or equal to max", path)
	}
}

func (v *validator) skipElement(start xml.StartElement) error {
	depth := 1
	for depth > 0 {
		token, err := v.decoder.Token()
		if err != nil {
			return err
		}
		switch token.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
	return nil
}

func (v *validator) add(format string, args ...any) {
	v.errs = append(v.errs, fmt.Sprintf(format, args...))
}

func isBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "0", "1", "true", "false", "yes", "no":
		return true
	default:
		return false
	}
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
