// Package main demonstrates the usage of GORM with JSONB queries for AAS Submodels.
// This example shows how to use the logical expression grammar to build type-safe
// queries against JSONB columns in PostgreSQL using GORM.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/eclipse-basyx/basyx-go-components/internal/common/model"
	"github.com/eclipse-basyx/basyx-go-components/internal/common/model/grammar"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  "user=postgres password=snoopy2002 dbname=smrepogorm port=5432 sslmode=disable",
		PreferSimpleProtocol: true, // disables implicit prepared statement usage
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // Enable SQL logging
	})
	if err != nil {
		panic("failed to connect database")
	}
	ctx := context.Background()

	// // Drop tables if they exist (for clean migration)
	// db.Migrator().DropTable(
	// 	&model.Extension{},
	// 	&model.Reference{},
	// 	&model.Key{},
	// 	&model.LangStringNameType{},
	// 	&model.LangStringTextType{},
	// 	&model.Submodel{},
	// )

	// // Migrate the schema
	// db.AutoMigrate(
	// 	&model.Submodel{},
	// 	&model.Extension{},
	// 	&model.Reference{},
	// 	&model.Key{},
	// 	&model.LangStringNameType{},
	// 	&model.LangStringTextType{},
	// )

	// // Create
	// db.Create(&model.Submodel{
	// 	ModelType: "Submodel",
	// 	Category:  "exampleCategory",
	// 	IdShort:   "exampleIdShort",
	// 	ID:        fmt.Sprintf("exampleID"),
	// 	Extension: []model.Extension{
	// 		{
	// 			ValueType: model.DATATYPEDEFXSD_XS_BOOLEAN,
	// 			Value:     "True",
	// 			Name:      "Test",
	// 			RefersTo: []model.Reference{
	// 				{
	// 					Type: model.REFERENCETYPES_EXTERNAL_REFERENCE,
	// 					Keys: []model.Key{
	// 						{
	// 							Type:  model.KEYTYPES_SUBMODEL,
	// 							Value: "exampleKeyValue",
	// 						},
	// 					},
	// 				},
	// 			},
	// 			SemanticID: &model.Reference{
	// 				Type: model.REFERENCETYPES_EXTERNAL_REFERENCE,
	// 				Keys: []model.Key{
	// 					{
	// 						Type:  model.KEYTYPES_SUBMODEL,
	// 						Value: "semantic",
	// 					},
	// 				},
	// 			},
	// 		},
	// 	},
	// 	DisplayName: []model.LangStringNameType{
	// 		{
	// 			Language: "de",
	// 			Text:     "Test",
	// 		},
	// 	},
	// 	Description: []model.LangStringTextType{
	// 		{
	// 			Language: "de",
	// 			Text:     "Test Beschreibung",
	// 		},
	// 	},
	// 	SemanticID: &model.Reference{
	// 		Type: model.REFERENCETYPES_EXTERNAL_REFERENCE,
	// 		Keys: []model.Key{
	// 			{
	// 				Type:  model.KEYTYPES_SUBMODEL,
	// 				Value: fmt.Sprintf("semantic"),
	// 			},
	// 		},
	// 		ReferredSemanticID: &model.Reference{
	// 			Type: model.REFERENCETYPES_MODEL_REFERENCE,
	// 			Keys: []model.Key{
	// 				{
	// 					Type:  model.KEYTYPES_ASSET_ADMINISTRATION_SHELL,
	// 					Value: "Nested",
	// 				},
	// 			},
	// 		},
	// 	},
	// 	SubmodelElements: []model.SubmodelElement{
	// 		&model.Property{
	// 			ModelType: "Property",
	// 			IdShort:   "ExampleProperty",
	// 			Value:     "ExampleValue",
	// 			ValueType: model.DATATYPEDEFXSD_XS_STRING,
	// 			SemanticID: &model.Reference{
	// 				Type: model.REFERENCETYPES_EXTERNAL_REFERENCE,
	// 				Keys: []model.Key{
	// 					{
	// 						Type:  model.KEYTYPES_ASSET_ADMINISTRATION_SHELL,
	// 						Value: fmt.Sprintf("PropertySemantic"),
	// 					},
	// 				},
	// 			},
	// 		},
	// 		&model.SubmodelElementList{
	// 			IdShort:   "col",
	// 			ModelType: "SubmodelElementList",
	// 			Value: []model.SubmodelElement{
	// 				&model.SubmodelElementCollection{
	// 					ModelType: "SubmodelElementCollection",
	// 					Value: []model.SubmodelElement{
	// 						&model.Property{
	// 							IdShort:   "PropertyA",
	// 							ModelType: "Property",
	// 							Value:     "a",
	// 							ValueType: model.DATATYPEDEFXSD_XS_STRING,
	// 						},
	// 						&model.Property{
	// 							IdShort:   "PropertyB",
	// 							ModelType: "Property",
	// 							Value:     "b",
	// 							ValueType: model.DATATYPEDEFXSD_XS_STRING,
	// 						},
	// 					},
	// 				},
	// 			},
	// 		},
	// 	},
	// })

	// for i := range 100_000 {
	// 	db.Create(&model.Submodel{
	// 		ModelType: "Submodel",
	// 		Category:  "exampleCategory",
	// 		IdShort:   "exampleIdShort",
	// 		ID:        fmt.Sprintf("exampleIDAdd_%d", i),
	// 		Extension: []model.Extension{
	// 			{
	// 				ValueType: model.DATATYPEDEFXSD_XS_BOOLEAN,
	// 				Value:     "True",
	// 				Name:      "Test",
	// 				RefersTo: []model.Reference{
	// 					{
	// 						Type: model.REFERENCETYPES_EXTERNAL_REFERENCE,
	// 						Keys: []model.Key{
	// 							{
	// 								Type:  model.KEYTYPES_SUBMODEL,
	// 								Value: "exampleKeyValue",
	// 							},
	// 						},
	// 					},
	// 				},
	// 				SemanticID: &model.Reference{
	// 					Type: model.REFERENCETYPES_EXTERNAL_REFERENCE,
	// 					Keys: []model.Key{
	// 						{
	// 							Type:  model.KEYTYPES_SUBMODEL,
	// 							Value: "semantic",
	// 						},
	// 					},
	// 				},
	// 			},
	// 		},
	// 		DisplayName: []model.LangStringNameType{
	// 			{
	// 				Language: "de",
	// 				Text:     "Test",
	// 			},
	// 		},
	// 		Description: []model.LangStringTextType{
	// 			{
	// 				Language: "de",
	// 				Text:     "Test Beschreibung",
	// 			},
	// 		},
	// 		SemanticID: &model.Reference{
	// 			Type: model.REFERENCETYPES_EXTERNAL_REFERENCE,
	// 			Keys: []model.Key{
	// 				{
	// 					Type:  model.KEYTYPES_SUBMODEL,
	// 					Value: fmt.Sprintf("semantic_%d", i),
	// 				},
	// 			},
	// 			ReferredSemanticID: &model.Reference{
	// 				Type: model.REFERENCETYPES_MODEL_REFERENCE,
	// 				Keys: []model.Key{
	// 					{
	// 						Type:  model.KEYTYPES_ASSET_ADMINISTRATION_SHELL,
	// 						Value: "Nested",
	// 					},
	// 				},
	// 			},
	// 		},
	// 		SubmodelElements: []model.SubmodelElement{
	// 			&model.Property{
	// 				ModelType: "Property",
	// 				IdShort:   "ExampleProperty",
	// 				Value:     "ExampleValue",
	// 				ValueType: model.DATATYPEDEFXSD_XS_STRING,
	// 				SemanticID: &model.Reference{
	// 					Type: model.REFERENCETYPES_EXTERNAL_REFERENCE,
	// 					Keys: []model.Key{
	// 						{
	// 							Type:  model.KEYTYPES_ASSET_ADMINISTRATION_SHELL,
	// 							Value: fmt.Sprintf("PropertySemantic_%d", i),
	// 						},
	// 					},
	// 				},
	// 			},
	// 			&model.SubmodelElementList{
	// 				IdShort:   "col",
	// 				ModelType: "SubmodelElementList",
	// 				Value: []model.SubmodelElement{
	// 					&model.SubmodelElementCollection{
	// 						ModelType: "SubmodelElementCollection",
	// 						Value: []model.SubmodelElement{
	// 							&model.Property{
	// 								IdShort:   "PropertyA",
	// 								Value:     "a",
	// 								ModelType: "Property",
	// 								ValueType: model.DATATYPEDEFXSD_XS_STRING,
	// 							},
	// 							&model.Property{
	// 								IdShort:   "PropertyB",
	// 								Value:     "c",
	// 								ModelType: "Property",
	// 								ValueType: model.DATATYPEDEFXSD_XS_STRING,
	// 							},
	// 						},
	// 					}, &model.SubmodelElementCollection{
	// 						Value: []model.SubmodelElement{
	// 							&model.Property{
	// 								IdShort:   "PropertyA",
	// 								Value:     "b",
	// 								ModelType: "Property",
	// 								ValueType: model.DATATYPEDEFXSD_XS_STRING,
	// 							},
	// 							&model.Property{
	// 								IdShort:   "PropertyB",
	// 								Value:     "b",
	// 								ModelType: "Property",
	// 								ValueType: model.DATATYPEDEFXSD_XS_STRING,
	// 							},
	// 						},
	// 					},
	// 				},
	// 			},
	// 		},
	// 	})
	// }
	start := time.Now().UnixMicro()

	// ==================================================================================
	// Example 1: Using LogicalExpression to query by idShort
	// ==================================================================================
	field1 := grammar.ModelStringPattern("$sm#idShort")
	value1 := grammar.StandardString("Identification_11")

	expr1 := grammar.LogicalExpression{
		Eq: []grammar.Value{
			{Field: &field1},
			{StrVal: &value1},
		},
	}

	sql1, args1, err := expr1.ToGORMWhere()
	if err != nil {
		panic(fmt.Sprintf("Failed to convert expression to GORM WHERE: %v", err))
	}

	// This generates SQL like: WHERE id_short = 'exampleIdShort'
	result1, err := gorm.G[model.Submodel](db).Where(sql1, args1...).Count(ctx, "*")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Example 1 - Count of submodels with idShort='hahhha': %d\n", result1)

	// ==================================================================================
	// Example 2: Query by semanticId (shorthand for keys[0].value)
	// ==================================================================================
	field2 := grammar.ModelStringPattern("$sm#semanticId")
	value2 := grammar.StandardString("RootLevelHAHA")

	expr2 := grammar.LogicalExpression{
		Eq: []grammar.Value{
			{Field: &field2},
			{StrVal: &value2},
		},
	}

	sql2, args2, err := expr2.ToGORMWhere()
	if err != nil {
		panic(fmt.Sprintf("Failed to convert expression to GORM WHERE: %v", err))
	}

	// This generates SQL like: WHERE semantic_id->'keys'->0->>'value' = 'semantic'
	result2, err := gorm.G[model.Submodel](db).Where(sql2, args2...).Count(ctx, "*")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Example 2 - Count of submodels with semanticId='RootLevelHAHA': %d\n", result2)

	// ==================================================================================
	// Example 3: Query by specific key index in semanticId
	// ==================================================================================
	field3 := grammar.ModelStringPattern("$sm#semanticId.keys[1].value")
	value3 := grammar.StandardString("RootLevel2")

	expr3 := grammar.LogicalExpression{
		Eq: []grammar.Value{
			{Field: &field3},
			{StrVal: &value3},
		},
	}

	sql3, args3, err := expr3.ToGORMWhere()
	if err != nil {
		panic(fmt.Sprintf("Failed to convert expression to GORM WHERE: %v", err))
	}

	// This generates SQL like: WHERE semantic_id->'keys'->0->>'value' = 'semantic'
	result3, err := gorm.G[model.Submodel](db).Where(sql3, args3...).Count(ctx, "*")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Example 3 - Count with specific key index [0]: %d\n", result3)

	// ==================================================================================
	// Example 4: Query with array wildcard (any key in the array)
	// ==================================================================================
	field4 := grammar.ModelStringPattern("$sm#semanticId.keys[].value")
	value4 := grammar.StandardString("RootLevelHAHA")

	expr4 := grammar.LogicalExpression{
		Eq: []grammar.Value{
			{Field: &field4},
			{StrVal: &value4},
		},
	}

	sql4, args4, err := expr4.ToGORMWhere()
	if err != nil {
		panic(fmt.Sprintf("Failed to convert expression to GORM WHERE: %v", err))
	}

	// This generates SQL like: WHERE semantic_id @? '$.keys[*] ? (@.value == "semantic")'
	result4, err := gorm.G[model.Submodel](db).Where(sql4, args4...).Count(ctx, "*")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Example 4 - Count with array wildcard []: %d\n", result4)

	// ==================================================================================
	// Example 5: Complex AND condition
	// ==================================================================================
	idShortField := grammar.ModelStringPattern("$sm#idShort")
	idShortValue := grammar.StandardString("IdentificationS")
	semanticField := grammar.ModelStringPattern("$sm#semanticId")
	semanticValue := grammar.StandardString("RootLevelHAHA")

	expr5 := grammar.LogicalExpression{
		And: []grammar.LogicalExpression{
			{
				Eq: []grammar.Value{
					{Field: &idShortField},
					{StrVal: &idShortValue},
				},
			},
			{
				Eq: []grammar.Value{
					{Field: &semanticField},
					{StrVal: &semanticValue},
				},
			},
		},
	}

	sql5, args5, err := expr5.ToGORMWhere()
	if err != nil {
		panic(fmt.Sprintf("Failed to convert expression to GORM WHERE: %v", err))
	}

	// This generates SQL like: WHERE (id_short = 'exampleIdShort' AND semantic_id->'keys'->0->>'value' = 'semantic')
	result5, err := gorm.G[model.Submodel](db).Where(sql5, args5...).Count(ctx, "*")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Example 5 - Count with AND condition: %d\n", result5)

	// ==================================================================================
	// Example 6: OR condition with numeric comparison
	// ==================================================================================
	idField6 := grammar.ModelStringPattern("$sm#id")
	idValue6 := grammar.StandardString("exampleID")
	categoryField6 := grammar.ModelStringPattern("$sm#idShort")
	categoryValue6 := grammar.StandardString("differentIdShort")

	expr6 := grammar.LogicalExpression{
		Or: []grammar.LogicalExpression{
			{
				Eq: []grammar.Value{
					{Field: &idField6},
					{StrVal: &idValue6},
				},
			},
			{
				Eq: []grammar.Value{
					{Field: &categoryField6},
					{StrVal: &categoryValue6},
				},
			},
		},
	}

	sql6, args6, err := expr6.ToGORMWhere()
	if err != nil {
		panic(fmt.Sprintf("Failed to convert expression to GORM WHERE: %v", err))
	}

	// This generates SQL like: WHERE (submodel_id = 'exampleID' OR id_short = 'differentIdShort')
	result6, err := gorm.G[model.Submodel](db).Where(sql6, args6...).Count(ctx, "*")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Example 6 - Count with OR condition: %d\n", result6)

	// ==================================================================================
	// Example 7: NOT condition
	// ==================================================================================
	notField7 := grammar.ModelStringPattern("$sm#idShort")
	notValue7 := grammar.StandardString("exampleIdShort")

	expr7 := grammar.LogicalExpression{
		Not: &grammar.LogicalExpression{
			Eq: []grammar.Value{
				{Field: &notField7},
				{StrVal: &notValue7},
			},
		},
	}

	sql7, args7, err := expr7.ToGORMWhere()
	if err != nil {
		panic(fmt.Sprintf("Failed to convert expression to GORM WHERE: %v", err))
	}

	// This generates SQL like: WHERE NOT (id_short = 'exampleIdShort')
	result7, err := gorm.G[model.Submodel](db).Where(sql7, args7...).Count(ctx, "*")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Example 7 - Count with NOT condition: %d\n", result7)

	endTime := time.Now().UnixMicro()
	fmt.Printf("\nTotal query time: %d microseconds (%d ms)\n", endTime-start, (endTime-start)/1000)

	// ==================================================================================
	// Original direct JSONB query example (for comparison)
	// ==================================================================================
	start2 := time.Now().UnixMicro()
	// Find Submodel with ID
	// submodel, err := gorm.G[model.Submodel](db).Where("submodel_elements @? '$[*] ? (@.idShort == \"NestedLevel18_Aaron\")'").Count(ctx, "*")
	// submodel, err := gorm.G[model.Submodel](db).Where("semantic_id	 @? '$.keys[*] ? (@.value == \"semantic_1\")'").First(ctx)
	submodel, err := gorm.G[model.Submodel](db).Where(`jsonb_path_exists(
	    submodel_elements,
	    '$[*].value[*] ? (
	      exists(@.value[*] ? (@.idShort == "PropertyA" && @.value == "a")) &&
	      exists(@.value[*] ? (@.idShort == "PropertyB" && @.value == "b"))
	    )'
	  )
	`).Count(ctx, "*")
	endTime2 := time.Now().UnixMicro()
	if err != nil {
		panic(err)
	}
	fmt.Printf("\nDirect JSONB query took %d microseconds\n", endTime2-start2)
	fmt.Printf("Direct JSONB query took %d ms\n", (endTime2-start2)/1000)
	fmt.Printf("Found Submodel Count: %d\n", submodel)

	// var json = jsoniter.ConfigCompatibleWithStandardLibrary
	// // to JSON
	// jsonData, err := json.MarshalIndent(submodel, "", "  ")
	// if err != nil {
	// 	panic(err)
	// }
	// println(string(jsonData))
}
