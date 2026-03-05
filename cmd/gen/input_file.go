package main

import (
	"bytes"
	"fmt"
)

func inputFileUploadCode(buffer *bytes.Buffer, expr, name string, required bool) {
	if !required {
		_, _ = fmt.Fprintf(buffer, "if %s != nil {\n", expr)
	} else {
		expr = "&" + expr
	}

	_, _ = fmt.Fprintf(buffer, `err = createFormFileFromInputFile(
	        writer,
			%s,
			%s,
		)
		if err != nil {
		    return
		}
		`, expr, name)

	if !required {
		_, _ = buffer.WriteString("}\n")
	}
}

func uploadNameExpr(name string, indexed bool) string {
	if indexed {
		return fmt.Sprintf("%q + strconv.Itoa(i)", name)
	}

	return fmt.Sprintf("%q", name)
}

func inputFileFieldsCode(buffer *bytes.Buffer, fields []Field, exprPrefix, namePrefix string, indexed bool) bool {
	found := false
	discriminator := ""

	for _, field := range fields {
		if field.Discriminator != "" {
			discriminator = field.Discriminator
		}

		expr := exprPrefix + "." + toTitle(field.Name)
		name := uploadNameExpr(namePrefix+"_"+discriminator+"_"+field.Name, indexed)

		if toType(field.Type, true) == "InputFile" {
			found = true
			inputFileUploadCode(buffer, expr, name, field.IsRequired)
		}
	}

	return found
}

func inputFileSubtypesCode(buffer *bytes.Buffer,
	subtypes []string, exprPrefix, namePrefix string, indexed bool, types map[string]Type,
) bool {
	found := false

	buffer.WriteString("switch {\n")

	for _, subtypeName := range subtypes {
		subtype, ok := types[subtypeName]
		if !ok {
			continue
		}

		expr := exprPrefix + "." + subtypeName
		name := namePrefix

		_, _ = fmt.Fprintf(buffer, "case %s.%s != nil:\n", exprPrefix, subtypeName)
		if inputFileFieldsCode(buffer, subtype.Fields, expr, name, indexed) {
			found = true
		}
	}

	buffer.WriteString("}\n")

	return found
}
