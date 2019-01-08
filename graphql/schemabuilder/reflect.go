package schemabuilder

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/samsarahq/thunder/graphql"
	"github.com/samsarahq/thunder/internal"
)

type argParser struct {
	FromJSON func(interface{}, reflect.Value) error
	Type     reflect.Type
}

func nilParseArguments(args interface{}) (interface{}, error) {
	if args == nil {
		return nil, nil
	}
	if args, ok := args.(map[string]interface{}); !ok || len(args) != 0 {
		return nil, graphql.NewSafeError("unexpected args")
	}
	return nil, nil
}

func (p *argParser) Parse(args interface{}) (interface{}, error) {
	if p == nil {
		return nilParseArguments(args)
	}
	parsed := reflect.New(p.Type).Elem()
	if err := p.FromJSON(args, parsed); err != nil {
		return nil, err
	}
	return parsed.Interface(), nil
}

// TODO: Enforce keys for items in lists, support compound keys

func makeGraphql(s string) string {
	var b bytes.Buffer
	for i, c := range s {
		if i == 0 {
			b.WriteRune(unicode.ToLower(c))
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}

func reverseGraphqlFieldName(s string) string {
	var b bytes.Buffer
	for i, c := range s {
		if i == 0 {
			b.WriteRune(unicode.ToUpper(c))
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}

var scalarArgParsers = map[reflect.Type]*argParser{
	reflect.TypeOf(bool(false)): {
		FromJSON: func(value interface{}, dest reflect.Value) error {
			asBool, ok := value.(bool)
			if !ok {
				return errors.New("not a bool")
			}
			dest.Set(reflect.ValueOf(asBool).Convert(dest.Type()))
			return nil
		},
	},
	reflect.TypeOf(float64(0)): {
		FromJSON: func(value interface{}, dest reflect.Value) error {
			asFloat, ok := value.(float64)
			if !ok {
				return errors.New("not a number")
			}
			dest.Set(reflect.ValueOf(asFloat).Convert(dest.Type()))
			return nil
		},
	},
	reflect.TypeOf(float32(0)): {
		FromJSON: func(value interface{}, dest reflect.Value) error {
			asFloat, ok := value.(float64)
			if !ok {
				return errors.New("not a number")
			}
			dest.Set(reflect.ValueOf(float32(asFloat)).Convert(dest.Type()))
			return nil
		},
	},
	reflect.TypeOf(int64(0)): {
		FromJSON: func(value interface{}, dest reflect.Value) error {
			asFloat, ok := value.(float64)
			if !ok {
				return errors.New("not a number")
			}
			dest.Set(reflect.ValueOf(int64(asFloat)).Convert(dest.Type()))
			return nil
		},
	},
	reflect.TypeOf(int32(0)): {
		FromJSON: func(value interface{}, dest reflect.Value) error {
			asFloat, ok := value.(float64)
			if !ok {
				return errors.New("not a number")
			}
			dest.Set(reflect.ValueOf(int32(asFloat)).Convert(dest.Type()))
			return nil
		},
	},
	reflect.TypeOf(int16(0)): {
		FromJSON: func(value interface{}, dest reflect.Value) error {
			asFloat, ok := value.(float64)
			if !ok {
				return errors.New("not a number")
			}
			dest.Set(reflect.ValueOf(int16(asFloat)).Convert(dest.Type()))
			return nil
		},
	},
	reflect.TypeOf(int8(0)): {
		FromJSON: func(value interface{}, dest reflect.Value) error {
			asFloat, ok := value.(float64)
			if !ok {
				return errors.New("not a number")
			}
			dest.Set(reflect.ValueOf(int8(asFloat)).Convert(dest.Type()))
			return nil
		},
	},
	reflect.TypeOf(uint64(0)): {
		FromJSON: func(value interface{}, dest reflect.Value) error {
			asFloat, ok := value.(float64)
			if !ok {
				return errors.New("not a number")
			}
			dest.Set(reflect.ValueOf(int64(asFloat)).Convert(dest.Type()))
			return nil
		},
	},
	reflect.TypeOf(uint32(0)): {
		FromJSON: func(value interface{}, dest reflect.Value) error {
			asFloat, ok := value.(float64)
			if !ok {
				return errors.New("not a number")
			}
			dest.Set(reflect.ValueOf(uint32(asFloat)).Convert(dest.Type()))
			return nil
		},
	},
	reflect.TypeOf(uint16(0)): {
		FromJSON: func(value interface{}, dest reflect.Value) error {
			asFloat, ok := value.(float64)
			if !ok {
				return errors.New("not a number")
			}
			dest.Set(reflect.ValueOf(uint16(asFloat)).Convert(dest.Type()))
			return nil
		},
	},
	reflect.TypeOf(uint8(0)): {
		FromJSON: func(value interface{}, dest reflect.Value) error {
			asFloat, ok := value.(float64)
			if !ok {
				return errors.New("not a number")
			}
			dest.Set(reflect.ValueOf(uint8(asFloat)).Convert(dest.Type()))
			return nil
		},
	},
	reflect.TypeOf(string("")): {
		FromJSON: func(value interface{}, dest reflect.Value) error {
			asString, ok := value.(string)
			if !ok {
				return errors.New("not a string")
			}
			dest.Set(reflect.ValueOf(asString).Convert(dest.Type()))
			return nil
		},
	},
	reflect.TypeOf([]byte{}): {
		FromJSON: func(value interface{}, dest reflect.Value) error {
			asString, ok := value.(string)
			if !ok {
				return errors.New("not a string")
			}
			bytes, err := base64.StdEncoding.DecodeString(asString)
			if err != nil {
				return err
			}
			dest.Set(reflect.ValueOf(bytes).Convert(dest.Type()))
			return nil
		},
	},
	reflect.TypeOf(time.Time{}): {
		FromJSON: func(value interface{}, dest reflect.Value) error {
			asString, ok := value.(string)
			if !ok {
				return errors.New("not a string")
			}
			asTime, err := time.Parse(time.RFC3339, asString)
			if err != nil {
				return errors.New("not an iso8601 time")
			}
			dest.Set(reflect.ValueOf(asTime).Convert(dest.Type()))
			return nil
		},
	},
}

func getScalarArgParser(typ reflect.Type) (*argParser, graphql.Type, bool) {
	for match, argParser := range scalarArgParsers {
		if internal.TypesIdenticalOrScalarAliases(match, typ) {
			name, ok := getScalar(typ)
			if !ok {
				panic(typ)
			}

			if typ != argParser.Type {
				// The scalar may be a type alias here,
				// so we annotate the parser to output the
				// alias instead of the underlying type.
				newParser := *argParser
				newParser.Type = typ
				argParser = &newParser
			}

			return argParser, &graphql.Scalar{Type: name}, true
		}
	}
	return nil, nil, false
}

func (sb *schemaBuilder) getEnumArgParser(typ reflect.Type) (*argParser, graphql.Type) {
	var values []string
	for mapping := range sb.enumMappings[typ].Map {
		values = append(values, mapping)
	}
	return &argParser{FromJSON: func(value interface{}, dest reflect.Value) error {
		asString, ok := value.(string)
		if !ok {
			return errors.New("not a string")
		}
		val, ok := sb.enumMappings[typ].Map[asString]
		if !ok {
			return fmt.Errorf("unknown enum value %v", asString)
		}
		dest.Set(reflect.ValueOf(val).Convert(dest.Type()))
		return nil
	}, Type: typ}, &graphql.Enum{Type: typ.Name(), Values: values, ReverseMap: sb.enumMappings[typ].ReverseMap}

}

func init() {
	for typ, arg := range scalarArgParsers {
		arg.Type = typ
	}
}

type argField struct {
	field    reflect.StructField
	parser   *argParser
	optional bool
}

func (sb *schemaBuilder) makeArgParser(typ reflect.Type) (*argParser, graphql.Type, error) {
	if typ.Kind() == reflect.Ptr {
		parser, argType, err := sb.makeArgParserInner(typ.Elem())
		if err != nil {
			return nil, nil, err
		}
		return wrapPtrParser(parser), argType, nil
	}

	parser, argType, err := sb.makeArgParserInner(typ)
	if err != nil {
		return nil, nil, err
	}
	return parser, &graphql.NonNull{Type: argType}, nil
}

func (sb *schemaBuilder) makeArgParserInner(typ reflect.Type) (*argParser, graphql.Type, error) {
	if sb.enumMappings[typ] != nil {
		parser, argType := sb.getEnumArgParser(typ)
		return parser, argType, nil
	}

	if parser, argType, ok := getScalarArgParser(typ); ok {
		return parser, argType, nil
	}

	switch typ.Kind() {
	case reflect.Struct:
		parser, argType, err := sb.makeStructParser(typ)
		if err != nil {
			return nil, nil, err
		}
		if argType.(*graphql.InputObject).Name == "" {
			return nil, nil, fmt.Errorf("bad type %s: should have a name", typ)
		}
		return parser, argType, nil
	case reflect.Slice:
		return sb.makeSliceParser(typ)
	default:
		return nil, nil, fmt.Errorf("bad arg type %s: should be struct, scalar, pointer, or a slice", typ)
	}
}

func wrapPtrParser(inner *argParser) *argParser {
	return &argParser{
		FromJSON: func(value interface{}, dest reflect.Value) error {
			if value == nil {
				// optional value
				return nil
			}

			ptr := reflect.New(inner.Type)
			if err := inner.FromJSON(value, ptr.Elem()); err != nil {
				return err
			}
			dest.Set(ptr)
			return nil
		},
		Type: reflect.PtrTo(inner.Type),
	}
}

func (sb *schemaBuilder) getStructObjectFields(typ reflect.Type) (*graphql.InputObject, map[string]argField, error) {
	// Check if the struct type is already cached
	if cached, ok := sb.typeCache[typ]; ok {
		return cached.argType, cached.fields, nil
	}

	fields := make(map[string]argField)
	argType := &graphql.InputObject{
		Name:        typ.Name(),
		InputFields: make(map[string]graphql.Type),
	}
	if argType.Name != "" {
		argType.Name += "_InputObject"
	}

	if typ.Kind() != reflect.Struct {
		return nil, nil, fmt.Errorf("expected struct but received type %s", typ.Name())
	}

	// Cache type information ahead of time to catch self-reference
	sb.typeCache[typ] = cachedType{argType, fields}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Anonymous {
			return nil, nil, fmt.Errorf("bad arg type %s: anonymous fields not supported", typ)
		}

		fieldInfo, err := parseGraphQLFieldInfo(field)
		if err != nil {
			return nil, nil, fmt.Errorf("bad type %s: %s", typ, err.Error())
		}
		if fieldInfo.Skipped {
			continue
		}

		if _, ok := fields[fieldInfo.Name]; ok {
			return nil, nil, fmt.Errorf("bad arg type %s: duplicate field %s", typ, fieldInfo.Name)
		}
		parser, fieldArgTyp, err := sb.makeArgParser(field.Type)
		if err != nil {
			return nil, nil, err
		}

		fields[fieldInfo.Name] = argField{
			field:  field,
			parser: parser,
		}
		argType.InputFields[fieldInfo.Name] = fieldArgTyp
	}

	return argType, fields, nil
}

// graphQLFieldInfo contains basic struct field information related to GraphQL.
type graphQLFieldInfo struct {
	// Skipped indicates that this field should not be included in GraphQL.
	Skipped bool

	// Name is the GraphQL field name that should be exposed for this field.
	Name string

	// KeyField indicates that this field should be treated as a Object Key field.
	KeyField bool
}

// parseGraphQLFieldInfo parses a struct field and returns a struct with the
// parsed information about the field (tag info, name, etc).
func parseGraphQLFieldInfo(field reflect.StructField) (*graphQLFieldInfo, error) {
	if field.PkgPath != "" {
		return &graphQLFieldInfo{Skipped: true}, nil
	}
	tags := strings.Split(field.Tag.Get("graphql"), ",")
	var name string
	if len(tags) > 0 {
		name = tags[0]
	}
	if name == "" {
		name = makeGraphql(field.Name)
	}
	if name == "-" {
		return &graphQLFieldInfo{Skipped: true}, nil
	}

	var key bool

	if len(tags) > 1 {
		for _, tag := range tags[1:] {
			if tag != "key" || key {
				return nil, fmt.Errorf("field %s has unexpected tag %s", name, tag)
			}
			key = true
		}
	}
	return &graphQLFieldInfo{Name: name, KeyField: key}, nil
}

func (sb *schemaBuilder) makeStructParser(typ reflect.Type) (*argParser, graphql.Type, error) {
	argType, fields, err := sb.getStructObjectFields(typ)
	if err != nil {
		return nil, nil, err
	}

	return &argParser{
		FromJSON: func(value interface{}, dest reflect.Value) error {
			asMap, ok := value.(map[string]interface{})
			if !ok {
				return errors.New("not an object")
			}

			for name, field := range fields {
				value := asMap[name]
				fieldDest := dest.FieldByIndex(field.field.Index)
				if err := field.parser.FromJSON(value, fieldDest); err != nil {
					return fmt.Errorf("%s: %s", name, err)
				}
			}
			for name := range asMap {
				if _, ok := fields[name]; !ok {
					return fmt.Errorf("unknown arg %s", name)
				}
			}

			return nil
		},
		Type: typ,
	}, argType, nil
}

func (sb *schemaBuilder) makeSliceParser(typ reflect.Type) (*argParser, graphql.Type, error) {
	inner, argType, err := sb.makeArgParser(typ.Elem())
	if err != nil {
		return nil, nil, err
	}

	return &argParser{
		FromJSON: func(value interface{}, dest reflect.Value) error {
			asSlice, ok := value.([]interface{})
			if !ok {
				return errors.New("not a list")
			}

			dest.Set(reflect.MakeSlice(typ, len(asSlice), len(asSlice)))

			for i, value := range asSlice {
				if err := inner.FromJSON(value, dest.Index(i)); err != nil {
					return err
				}
			}

			return nil
		},
		Type: typ,
	}, &graphql.List{Type: argType}, nil
}

type schemaBuilder struct {
	types        map[reflect.Type]graphql.Type
	objects      map[reflect.Type]*Object
	enumMappings map[reflect.Type]*EnumMapping
	typeCache    map[reflect.Type]cachedType // typeCache maps Go types to GraphQL datatypes
}

type EnumMapping struct {
	Map        map[string]interface{}
	ReverseMap map[interface{}]string
}

// cachedType is a container for GraphQL datatype and the list of its fields
type cachedType struct {
	argType *graphql.InputObject
	fields  map[string]argField
}

var errType reflect.Type
var contextType reflect.Type
var selectionSetType reflect.Type

func init() {
	var err error
	errType = reflect.TypeOf(&err).Elem()
	var context context.Context
	contextType = reflect.TypeOf(&context).Elem()
	var selectionSet *graphql.SelectionSet
	selectionSetType = reflect.TypeOf(selectionSet)
}

func (sb *schemaBuilder) buildField(field reflect.StructField) (*graphql.Field, error) {
	retType, err := sb.getType(field.Type)
	if err != nil {
		return nil, err
	}

	return &graphql.Field{
		Resolve: func(ctx context.Context, source, args interface{}, selectionSet *graphql.SelectionSet) (interface{}, error) {
			value := reflect.ValueOf(source)
			if value.Kind() == reflect.Ptr {
				value = value.Elem()
			}
			return value.FieldByIndex(field.Index).Interface(), nil
		},
		Type:           retType,
		ParseArguments: nilParseArguments,
	}, nil
}

func (sb *schemaBuilder) buildUnionStruct(typ reflect.Type) error {
	var name string
	var description string

	if name == "" {
		name = typ.Name()
		if name == "" {
			return fmt.Errorf("bad type %s: should have a name", typ)
		}
	}

	union := &graphql.Union{
		Name:        name,
		Description: description,
		Types:       make(map[string]*graphql.Object),
	}
	sb.types[typ] = union

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" || (field.Anonymous && field.Type == unionType) {
			continue
		}

		if !field.Anonymous {
			return fmt.Errorf("bad type %s: union type member types must be anonymous", name)
		}

		typ, err := sb.getType(field.Type)
		if err != nil {
			return err
		}

		obj, ok := typ.(*graphql.Object)
		if !ok {
			return fmt.Errorf("bad type %s: union type member must be a pointer to a struct, received %s", name, typ.String())
		}

		if union.Types[obj.Name] != nil {
			return fmt.Errorf("bad type %s: union type member may only appear once", name)
		}

		union.Types[obj.Name] = obj
	}
	return nil
}

func (sb *schemaBuilder) buildStruct(typ reflect.Type) error {
	if sb.types[typ] != nil {
		return nil
	}

	if typ == unionType {
		return fmt.Errorf("schemabuilder.Union can only be used as an embedded anonymous non-pointer struct")
	}

	if hasUnionMarkerEmbedded(typ) {
		return sb.buildUnionStruct(typ)
	}

	var name string
	var description string
	var methods Methods
	var objectKey string
	if object, ok := sb.objects[typ]; ok {
		name = object.Name
		description = object.Description
		methods = object.Methods
		objectKey = object.key
	}

	if name == "" {
		name = typ.Name()
		if name == "" {
			return fmt.Errorf("bad type %s: should have a name", typ)
		}
	}

	object := &graphql.Object{
		Name:        name,
		Description: description,
		Fields:      make(map[string]*graphql.Field),
	}
	sb.types[typ] = object

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldInfo, err := parseGraphQLFieldInfo(field)
		if err != nil {
			return fmt.Errorf("bad type %s: %s", typ, fieldInfo.Name)
		}
		if fieldInfo.Skipped {
			continue
		}

		if _, ok := object.Fields[fieldInfo.Name]; ok {
			return fmt.Errorf("bad type %s: two fields named %s", typ, fieldInfo.Name)
		}

		built, err := sb.buildField(field)
		if err != nil {
			return fmt.Errorf("bad field %s on type %s: %s", fieldInfo.Name, typ, err)
		}
		object.Fields[fieldInfo.Name] = built
		if fieldInfo.KeyField {
			if object.Key != nil {
				return fmt.Errorf("bad type %s: multiple key fields", typ)
			}
			if !isScalarType(built.Type) {
				return fmt.Errorf("bad type %s: key type must be scalar, got %T", typ, built.Type)
			}
			object.Key = built.Resolve
		}
	}

	var names []string
	for name := range methods {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		method := methods[name]

		if method.Paginated {
			typedField, err := sb.buildPaginatedField(typ, method)
			if err != nil {
				return err
			}
			object.Fields[name] = typedField
			continue
		}

		built, err := sb.buildFunction(typ, method)
		if err != nil {
			return fmt.Errorf("bad method %s on type %s: %s", name, typ, err)
		}
		object.Fields[name] = built
	}

	if objectKey != "" {
		keyPtr, ok := object.Fields[objectKey]
		if !ok {
			return fmt.Errorf("key field doesn't exist on object")
		}

		if !isScalarType(keyPtr.Type) {
			return fmt.Errorf("bad type %s: key type must be scalar, got %s", typ, keyPtr.Type.String())
		}
		object.Key = keyPtr.Resolve
	}

	return nil
}

func isScalarType(typ graphql.Type) bool {
	if nonNull, ok := typ.(*graphql.NonNull); ok {
		typ = nonNull.Type
	}
	if _, ok := typ.(*graphql.Scalar); !ok {
		return false
	}
	return true
}

var scalars = map[reflect.Type]string{
	reflect.TypeOf(bool(false)): "bool",
	reflect.TypeOf(int(0)):      "int",
	reflect.TypeOf(int8(0)):     "int8",
	reflect.TypeOf(int16(0)):    "int16",
	reflect.TypeOf(int32(0)):    "int32",
	reflect.TypeOf(int64(0)):    "int64",
	reflect.TypeOf(uint(0)):     "uint",
	reflect.TypeOf(uint8(0)):    "uint8",
	reflect.TypeOf(uint16(0)):   "uint16",
	reflect.TypeOf(uint32(0)):   "uint32",
	reflect.TypeOf(uint64(0)):   "uint64",
	reflect.TypeOf(float32(0)):  "float32",
	reflect.TypeOf(float64(0)):  "float64",
	reflect.TypeOf(string("")):  "string",
	reflect.TypeOf(time.Time{}): "Time",
	reflect.TypeOf([]byte{}):    "bytes",
}

func getScalar(typ reflect.Type) (string, bool) {
	for match, name := range scalars {
		if internal.TypesIdenticalOrScalarAliases(match, typ) {
			return name, true
		}
	}
	return "", false
}

func (sb *schemaBuilder) getEnum(typ reflect.Type) (string, []string, bool) {
	if sb.enumMappings[typ] != nil {
		var values []string
		for mapping := range sb.enumMappings[typ].Map {
			values = append(values, mapping)
		}
		return typ.Name(), values, true
	}
	return "", nil, false
}

func hasUnionMarkerEmbedded(typ reflect.Type) bool {
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Anonymous && field.Type == unionType {
			return true
		}
	}
	return false
}

func (sb *schemaBuilder) getType(t reflect.Type) (graphql.Type, error) {
	// Support scalars and optional scalars. Scalars have precedence over structs
	// to have eg. time.Time function as a scalar.
	if typ, values, ok := sb.getEnum(t); ok {
		return &graphql.NonNull{Type: &graphql.Enum{Type: typ, Values: values, ReverseMap: sb.enumMappings[t].ReverseMap}}, nil
	}

	if typ, ok := getScalar(t); ok {
		return &graphql.NonNull{Type: &graphql.Scalar{Type: typ}}, nil
	}
	if t.Kind() == reflect.Ptr {
		if typ, ok := getScalar(t.Elem()); ok {
			return &graphql.Scalar{Type: typ}, nil // XXX: prefix typ with "*"
		}
	}

	// Structs
	if t.Kind() == reflect.Struct {
		if err := sb.buildStruct(t); err != nil {
			return nil, err
		}
		return &graphql.NonNull{Type: sb.types[t]}, nil
	}
	if t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Struct {
		if err := sb.buildStruct(t.Elem()); err != nil {
			return nil, err
		}
		return sb.types[t.Elem()], nil
	}

	switch t.Kind() {
	case reflect.Slice:
		typ, err := sb.getType(t.Elem())
		if err != nil {
			return nil, err
		}

		// Wrap all slice elements in NonNull.
		if _, ok := typ.(*graphql.NonNull); !ok {
			typ = &graphql.NonNull{Type: typ}
		}

		return &graphql.NonNull{Type: &graphql.List{Type: typ}}, nil

	default:
		return nil, fmt.Errorf("bad type %s: should be a scalar, slice, or struct type", t)
	}
}
