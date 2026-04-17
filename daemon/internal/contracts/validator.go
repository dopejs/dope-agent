package contracts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type Validator struct {
	rootDir string
	cache   map[string]any
}

func NewValidator(rootDir string) *Validator {
	return &Validator{
		rootDir: rootDir,
		cache:   make(map[string]any),
	}
}

func (v *Validator) ValidateRelative(schemaPath string, document []byte) error {
	absolutePath := filepath.Join(v.rootDir, filepath.Clean(schemaPath))
	return v.ValidateAbsolute(absolutePath, document)
}

func (v *Validator) ValidateAbsolute(schemaPath string, document []byte) error {
	schema, err := v.loadSchema(schemaPath)
	if err != nil {
		return err
	}

	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("decode document %s: %w", schemaPath, err)
	}

	if err := v.validateSchema(schemaPath, schemaPath, schema, value, "$"); err != nil {
		return err
	}
	return nil
}

func (v *Validator) loadSchema(schemaPath string) (any, error) {
	if cached, ok := v.cache[schemaPath]; ok {
		return cached, nil
	}

	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("read schema %s: %w", schemaPath, err)
	}

	var schema any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, fmt.Errorf("decode schema %s: %w", schemaPath, err)
	}

	v.cache[schemaPath] = schema
	return schema, nil
}

func (v *Validator) validateSchema(rootSchemaPath, currentSchemaPath string, schema any, value any, fieldPath string) error {
	schemaMap, ok := schema.(map[string]any)
	if !ok {
		return fmt.Errorf("%s: unsupported schema node in %s", fieldPath, currentSchemaPath)
	}

	if ref, ok := schemaMap["$ref"].(string); ok {
		resolvedRoot, resolvedCurrent, resolvedSchema, err := v.resolveRef(rootSchemaPath, currentSchemaPath, ref)
		if err != nil {
			return fmt.Errorf("%s: resolve ref %q: %w", fieldPath, ref, err)
		}
		if err := v.validateSchema(resolvedRoot, resolvedCurrent, resolvedSchema, value, fieldPath); err != nil {
			return err
		}
	}

	if allOf, ok := schemaMap["allOf"].([]any); ok {
		for _, item := range allOf {
			if err := v.validateSchema(rootSchemaPath, currentSchemaPath, item, value, fieldPath); err != nil {
				return err
			}
		}
	}

	if enum, ok := schemaMap["enum"].([]any); ok && !containsJSONValue(enum, value) {
		return fmt.Errorf("%s: value %v is not in enum %v", fieldPath, value, enum)
	}
	if constant, ok := schemaMap["const"]; ok && !jsonValueEqual(constant, value) {
		return fmt.Errorf("%s: value %v does not match const %v", fieldPath, value, constant)
	}

	if typeDecl, ok := schemaMap["type"]; ok {
		matched, err := matchesDeclaredType(typeDecl, value)
		if err != nil {
			return fmt.Errorf("%s: %w", fieldPath, err)
		}
		if !matched {
			return fmt.Errorf("%s: value of type %T does not match schema type %v", fieldPath, value, typeDecl)
		}
	}

	if format, ok := schemaMap["format"].(string); ok && value != nil {
		if err := validateFormat(fieldPath, format, value); err != nil {
			return err
		}
	}

	if minimum, ok := schemaMap["minimum"]; ok && value != nil {
		if err := validateMinimum(fieldPath, minimum, value); err != nil {
			return err
		}
	}

	if minLength, ok := schemaMap["minLength"]; ok && value != nil {
		if err := validateMinLength(fieldPath, minLength, value); err != nil {
			return err
		}
	}

	if minItems, ok := schemaMap["minItems"]; ok && value != nil {
		if err := validateMinItems(fieldPath, minItems, value); err != nil {
			return err
		}
	}

	if objectValue, ok := value.(map[string]any); ok {
		if required, ok := schemaMap["required"].([]any); ok {
			for _, item := range required {
				key, ok := item.(string)
				if !ok {
					return fmt.Errorf("%s: invalid required key declaration %v", fieldPath, item)
				}
				if _, exists := objectValue[key]; !exists {
					return fmt.Errorf("%s.%s: required property is missing", fieldPath, key)
				}
			}
		}

		properties := map[string]any{}
		if rawProperties, ok := schemaMap["properties"].(map[string]any); ok {
			properties = rawProperties
		}

		if additionalProperties, ok := schemaMap["additionalProperties"].(bool); ok && !additionalProperties {
			for key := range objectValue {
				if _, allowed := properties[key]; !allowed {
					return fmt.Errorf("%s.%s: additional property is not allowed", fieldPath, key)
				}
			}
		}

		for key, propertySchema := range properties {
			propertyValue, exists := objectValue[key]
			if !exists {
				continue
			}
			if err := v.validateSchema(rootSchemaPath, currentSchemaPath, propertySchema, propertyValue, fieldPath+"."+key); err != nil {
				return err
			}
		}
	}

	if arrayValue, ok := value.([]any); ok {
		if itemSchema, ok := schemaMap["items"]; ok {
			for index, item := range arrayValue {
				if err := v.validateSchema(rootSchemaPath, currentSchemaPath, itemSchema, item, fmt.Sprintf("%s[%d]", fieldPath, index)); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (v *Validator) resolveRef(rootSchemaPath, currentSchemaPath, ref string) (string, string, any, error) {
	if strings.HasPrefix(ref, "#/") {
		rootSchema, err := v.loadSchema(rootSchemaPath)
		if err != nil {
			return "", "", nil, err
		}
		resolved, err := resolveJSONPointer(rootSchema, strings.TrimPrefix(ref, "#"))
		if err != nil {
			return "", "", nil, err
		}
		return rootSchemaPath, currentSchemaPath, resolved, nil
	}

	if strings.HasPrefix(ref, "#") {
		return "", "", nil, fmt.Errorf("unsupported local ref %q", ref)
	}

	referencePath := ref
	pointer := ""
	if index := strings.Index(ref, "#"); index >= 0 {
		referencePath = ref[:index]
		pointer = ref[index:]
	}

	targetPath := filepath.Clean(filepath.Join(filepath.Dir(currentSchemaPath), referencePath))
	targetSchema, err := v.loadSchema(targetPath)
	if err != nil {
		return "", "", nil, err
	}

	if pointer != "" {
		resolved, err := resolveJSONPointer(targetSchema, strings.TrimPrefix(pointer, "#"))
		if err != nil {
			return "", "", nil, err
		}
		return targetPath, targetPath, resolved, nil
	}

	return targetPath, targetPath, targetSchema, nil
}

func resolveJSONPointer(node any, pointer string) (any, error) {
	if pointer == "" {
		return node, nil
	}

	parts := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	current := node
	for _, raw := range parts {
		part := strings.ReplaceAll(strings.ReplaceAll(raw, "~1", "/"), "~0", "~")
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[part]
			if !ok {
				return nil, fmt.Errorf("pointer segment %q not found", part)
			}
			current = next
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("pointer segment %q is not an array index", part)
			}
			if index < 0 || index >= len(typed) {
				return nil, fmt.Errorf("pointer index %d out of range", index)
			}
			current = typed[index]
		default:
			return nil, fmt.Errorf("pointer segment %q cannot be resolved through %T", part, current)
		}
	}

	return current, nil
}

func containsJSONValue(items []any, value any) bool {
	for _, item := range items {
		if jsonValueEqual(item, value) {
			return true
		}
	}
	return false
}

func jsonValueEqual(left, right any) bool {
	left = normalizeJSONValue(left)
	right = normalizeJSONValue(right)
	return reflect.DeepEqual(left, right)
}

func normalizeJSONValue(value any) any {
	switch typed := value.(type) {
	case json.Number:
		if integer, err := typed.Int64(); err == nil {
			return integer
		}
		if number, err := typed.Float64(); err == nil {
			return number
		}
		return typed.String()
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, normalizeJSONValue(item))
		}
		return items
	case map[string]any:
		items := make(map[string]any, len(typed))
		for key, item := range typed {
			items[key] = normalizeJSONValue(item)
		}
		return items
	default:
		return value
	}
}

func matchesDeclaredType(typeDecl any, value any) (bool, error) {
	switch typed := typeDecl.(type) {
	case string:
		return matchesSingleType(typed, value), nil
	case []any:
		for _, item := range typed {
			name, ok := item.(string)
			if !ok {
				return false, fmt.Errorf("invalid type declaration %v", item)
			}
			if matchesSingleType(name, value) {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("unsupported type declaration %v", typeDecl)
	}
}

func matchesSingleType(typeName string, value any) bool {
	switch typeName {
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "integer":
		if value == nil {
			return false
		}
		_, err := toInt64(value)
		return err == nil
	case "number":
		if value == nil {
			return false
		}
		_, err := toFloat64(value)
		return err == nil
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "null":
		return value == nil
	default:
		return false
	}
}

func validateFormat(fieldPath, format string, value any) error {
	switch format {
	case "date-time":
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s: date-time format requires string value", fieldPath)
		}
		if _, err := time.Parse(time.RFC3339Nano, text); err != nil {
			return fmt.Errorf("%s: invalid date-time %q", fieldPath, text)
		}
	}
	return nil
}

func validateMinimum(fieldPath string, minimum any, value any) error {
	minFloat, err := toFloat64(minimum)
	if err != nil {
		return fmt.Errorf("%s: invalid schema minimum %v", fieldPath, minimum)
	}
	valueFloat, err := toFloat64(value)
	if err != nil {
		return fmt.Errorf("%s: minimum requires numeric value", fieldPath)
	}
	if valueFloat < minFloat {
		return fmt.Errorf("%s: value %v is smaller than minimum %v", fieldPath, value, minFloat)
	}
	return nil
}

func validateMinLength(fieldPath string, minLength any, value any) error {
	requiredLength, err := toInt64(minLength)
	if err != nil {
		return fmt.Errorf("%s: invalid schema minLength %v", fieldPath, minLength)
	}
	text, ok := value.(string)
	if !ok {
		return fmt.Errorf("%s: minLength requires string value", fieldPath)
	}
	if int64(len(text)) < requiredLength {
		return fmt.Errorf("%s: string length %d is smaller than minLength %d", fieldPath, len(text), requiredLength)
	}
	return nil
}

func validateMinItems(fieldPath string, minItems any, value any) error {
	requiredLength, err := toInt64(minItems)
	if err != nil {
		return fmt.Errorf("%s: invalid schema minItems %v", fieldPath, minItems)
	}
	items, ok := value.([]any)
	if !ok {
		return fmt.Errorf("%s: minItems requires array value", fieldPath)
	}
	if int64(len(items)) < requiredLength {
		return fmt.Errorf("%s: array length %d is smaller than minItems %d", fieldPath, len(items), requiredLength)
	}
	return nil
}

func toFloat64(value any) (float64, error) {
	switch typed := value.(type) {
	case float64:
		return typed, nil
	case float32:
		return float64(typed), nil
	case int:
		return float64(typed), nil
	case int64:
		return float64(typed), nil
	case int32:
		return float64(typed), nil
	case json.Number:
		return typed.Float64()
	default:
		return 0, fmt.Errorf("not a number: %T", value)
	}
}

func toInt64(value any) (int64, error) {
	switch typed := value.(type) {
	case int:
		return int64(typed), nil
	case int64:
		return typed, nil
	case int32:
		return int64(typed), nil
	case float64:
		if typed != float64(int64(typed)) {
			return 0, fmt.Errorf("not an integer: %v", typed)
		}
		return int64(typed), nil
	case json.Number:
		return typed.Int64()
	default:
		return 0, fmt.Errorf("not an integer: %T", value)
	}
}
