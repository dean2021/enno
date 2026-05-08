package enno

type Schema struct {
	properties map[string]any
	required   []string
}

func SchemaObject() *Schema {
	return &Schema{properties: map[string]any{}}
}

func (s *Schema) StringProp(name string) *Schema {
	return s.prop(name, map[string]any{"type": "string"})
}

func (s *Schema) IntegerProp(name string) *Schema {
	return s.prop(name, map[string]any{"type": "integer"})
}

func (s *Schema) BooleanProp(name string) *Schema {
	return s.prop(name, map[string]any{"type": "boolean"})
}

func (s *Schema) EnumProp(name string, values ...string) *Schema {
	enum := make([]string, len(values))
	copy(enum, values)
	return s.prop(name, map[string]any{
		"type": "string",
		"enum": enum,
	})
}

func (s *Schema) Required(names ...string) *Schema {
	if s == nil {
		return s
	}
	s.required = append(s.required, names...)
	return s
}

func (s *Schema) Properties() map[string]any {
	if s == nil || len(s.properties) == 0 {
		return nil
	}
	properties := make(map[string]any, len(s.properties))
	for name, property := range s.properties {
		properties[name] = cloneSchemaValue(property)
	}
	return properties
}

func (s *Schema) RequiredFields() []string {
	if s == nil || len(s.required) == 0 {
		return nil
	}
	return append([]string(nil), s.required...)
}

func (s *Schema) prop(name string, property map[string]any) *Schema {
	if s == nil {
		return s
	}
	if s.properties == nil {
		s.properties = map[string]any{}
	}
	s.properties[name] = property
	return s
}

func cloneSchemaValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		cloned := make(map[string]any, len(v))
		for key, item := range v {
			cloned[key] = cloneSchemaValue(item)
		}
		return cloned
	case []string:
		return append([]string(nil), v...)
	case []any:
		cloned := make([]any, len(v))
		for i, item := range v {
			cloned[i] = cloneSchemaValue(item)
		}
		return cloned
	default:
		return value
	}
}
