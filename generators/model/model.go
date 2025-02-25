package model

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/dizzyfool/genna/model"
	"github.com/dizzyfool/genna/util"
)

// TemplatePackage stores package info
type TemplatePackage struct {
	Package string

	HasImports bool
	Imports    []string

	Entities []TemplateEntity
}

// NewTemplatePackage creates a package for template
func NewTemplatePackage(entities []model.Entity, options Options) TemplatePackage {
	imports := util.NewSet()

	models := make([]TemplateEntity, len(entities))
	for i, entity := range entities {
		for _, imp := range entity.Imports {
			imports.Add(imp)
		}

		models[i] = NewTemplateEntity(entity, options)
	}

	return TemplatePackage{
		Package: options.Package,

		HasImports: imports.Len() > 0,
		Imports:    imports.Elements(),

		Entities: models,
	}
}

// TemplateEntity stores struct info
type TemplateEntity struct {
	model.Entity

	Tag template.HTML

	NoAlias bool
	Alias   string

	Columns []TemplateColumn

	HasRelations bool
	Relations    []TemplateRelation
}

// NewTemplateEntity creates an entity for template
func NewTemplateEntity(entity model.Entity, options Options) TemplateEntity {
	if entity.HasMultiplePKs() {
		options.KeepPK = true
	}

	columns := make([]TemplateColumn, len(entity.Columns))
	for i, column := range entity.Columns {
		columns[i] = NewTemplateColumn(entity, column, options)
	}

	relations := make([]TemplateRelation, len(entity.Relations))
	for i, relation := range entity.Relations {
		relations[i] = NewTemplateRelation(relation)
	}

	tags := util.NewAnnotation()
	tags.AddTag("sql", util.Quoted(entity.PGFullName, true))
	if !options.NoAlias {
		tags.AddTag("sql", fmt.Sprintf("alias:%s", util.DefaultAlias))
	}

	if !options.NoDiscard {
		// leading comma is required
		tags.AddTag("pg", ",discard_unknown_columns")
	}

	return TemplateEntity{
		Entity: entity,
		Tag:    template.HTML(fmt.Sprintf("`%s`", tags.String())),

		NoAlias: options.NoAlias,
		Alias:   util.DefaultAlias,

		Columns: columns,

		HasRelations: len(relations) > 0,
		Relations:    relations,
	}
}

// TemplateColumn stores column info
type TemplateColumn struct {
	model.Column

	Tag     template.HTML
	Comment template.HTML
}

// NewTemplateColumn creates a column for template
func NewTemplateColumn(entity model.Entity, column model.Column, options Options) TemplateColumn {
	if !options.KeepPK && column.IsPK {
		column.GoName = util.ID
	}

	if column.PGType == model.TypePGJSON || column.PGType == model.TypePGJSONB {
		if typ, ok := jsonType(options.JSONTypes, entity.PGSchema, entity.PGName, column.PGName); ok {
			column.Type = typ
		}
	}

	comment := ""
	tags := util.NewAnnotation()
	tags.AddTag("sql", column.PGName)

	// pk tag
	if column.IsPK {
		tags.AddTag("sql", "pk")
	}

	// types tag
	if column.PGType == model.TypePGHstore {
		tags.AddTag("sql", "hstore")
	} else if column.IsArray {
		tags.AddTag("sql", "array")
	}
	if column.PGType == model.TypePGUuid {
		tags.AddTag("sql", "type:uuid")
	}

	// nullable tag
	if !column.Nullable && !column.IsPK {
		tags.AddTag("sql", "notnull")
	}

	// soft_delete tag
	if options.SoftDelete == column.PGName && column.Nullable && column.GoType == model.TypeTime && !column.IsArray {
		tags.AddTag("pg", ",soft_delete")
	}

	// ignore tag
	if column.GoType == model.TypeInterface {
		comment = "// unsupported"
		tags = util.NewAnnotation().AddTag("sql", "-")
	}

	return TemplateColumn{
		Column: column,

		Tag:     template.HTML(fmt.Sprintf("`%s`", tags.String())),
		Comment: template.HTML(comment),
	}
}

// TemplateRelation stores relation info
type TemplateRelation struct {
	model.Relation

	Tag     template.HTML
	Comment template.HTML
}

// NewTemplateRelation creates relation for template
func NewTemplateRelation(relation model.Relation) TemplateRelation {
	comment := ""
	tags := util.NewAnnotation().AddTag("pg", "fk:"+strings.Join(relation.FKFields, ","))
	if len(relation.FKFields) > 1 {
		comment = "// unsupported"
		tags.AddTag("sql", "-")
	}

	return TemplateRelation{
		Relation: relation,

		Tag:     template.HTML(fmt.Sprintf("`%s`", tags.String())),
		Comment: template.HTML(comment),
	}
}

func jsonType(mp map[string]string, schema, table, field string) (string, bool) {
	if mp == nil {
		return "", false
	}

	patterns := [][3]string{
		{schema, table, field},
		{schema, "*", field},
		{schema, table, "*"},
		{schema, "*", "*"},
	}

	var names []string
	for _, parts := range patterns {
		names = append(names, fmt.Sprintf("%s.%s", util.Join(parts[0], parts[1]), parts[2]))
		names = append(names, fmt.Sprintf("%s.%s", util.JoinF(parts[0], parts[1]), parts[2]))
	}
	names = append(names, util.Join(schema, table), "*")

	for _, name := range names {
		if v, ok := mp[name]; ok {
			return v, true
		}
	}

	return "", false
}
