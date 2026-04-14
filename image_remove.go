package asposepdf

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// serializeContentOps converts parsed operators back to content stream bytes.
func serializeContentOps(ops []contentOp) []byte {
	var buf bytes.Buffer
	for _, op := range ops {
		if op.Operator == "BI" {
			// Inline image: write BI, dict key/values, ID, data, EI.
			serializeInlineImage(&buf, op)
			continue
		}
		for i, operand := range op.Operands {
			if i > 0 {
				buf.WriteByte(' ')
			}
			serializeOperand(&buf, operand)
		}
		if len(op.Operands) > 0 {
			buf.WriteByte(' ')
		}
		buf.WriteString(op.Operator)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

func serializeOperand(buf *bytes.Buffer, v pdfValue) {
	switch val := v.(type) {
	case int:
		buf.WriteString(strconv.Itoa(val))
	case float64:
		s := strconv.FormatFloat(val, 'f', 4, 64)
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
		buf.WriteString(s)
	case pdfName:
		buf.WriteString(string(val))
	case string:
		buf.WriteByte('(')
		buf.WriteString(escapeLiteral(val))
		buf.WriteByte(')')
	case pdfArray:
		buf.WriteByte('[')
		for i, item := range val {
			if i > 0 {
				buf.WriteByte(' ')
			}
			serializeOperand(buf, item)
		}
		buf.WriteByte(']')
	case pdfDict:
		buf.WriteString("<<")
		for k, dv := range val {
			buf.WriteString(k)
			buf.WriteByte(' ')
			serializeOperand(buf, dv)
			buf.WriteByte(' ')
		}
		buf.WriteString(">>")
	case bool:
		if val {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case pdfNull:
		buf.WriteString("null")
	default:
		fmt.Fprintf(buf, "%v", val)
	}
}

func serializeInlineImage(buf *bytes.Buffer, op contentOp) {
	if len(op.Operands) < 2 {
		return
	}
	buf.WriteString("BI\n")
	if dict, ok := op.Operands[0].(pdfDict); ok {
		for k, v := range dict {
			buf.WriteString(k)
			buf.WriteByte(' ')
			serializeOperand(buf, v)
			buf.WriteByte('\n')
		}
	}
	buf.WriteString("ID ")
	if data, ok := op.Operands[1].(string); ok {
		buf.WriteString(data)
	}
	buf.WriteString("\nEI\n")
}
