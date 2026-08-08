package dayzinit

type declarationKind uint8

const (
	declarationFunction declarationKind = iota
	declarationClass
	declarationEnum
	declarationVariable
	declarationTypedef
)

type typeRef struct {
	Name       string
	Arguments  []typeRef
	ArrayDepth int
	Span       Span
}

func (reference typeRef) String() string {
	result := reference.Name
	if len(reference.Arguments) > 0 {
		result += "<"
		for index, argument := range reference.Arguments {
			if index > 0 {
				result += ","
			}
			result += argument.String()
		}
		result += ">"
	}
	for count := 0; count < reference.ArrayDepth; count++ {
		result += "[]"
	}
	return result
}

type declaration struct {
	Kind        declarationKind
	Name        string
	Type        typeRef
	Base        *typeRef
	Modifiers   []string
	Parameters  []parameter
	Variables   []variable
	Members     []*declaration
	EnumMembers []enumMember
	Body        *statement
	HasBody     bool
	Span        Span
}

type parameter struct {
	Name      string
	Type      typeRef
	Modifiers []string
	Default   *expression
	Span      Span
}

type variable struct {
	Name        string
	ArrayDepth  int
	Initializer *expression
	Span        Span
}

type enumMember struct {
	Name  string
	Value *expression
	Span  Span
}

type statementKind uint8

const (
	statementBlock statementKind = iota
	statementEmpty
	statementDeclaration
	statementExpression
	statementIf
	statementSwitch
	statementWhile
	statementFor
	statementForeach
	statementReturn
	statementBreak
	statementContinue
	statementDelete
)

type statement struct {
	Kind        statementKind
	Span        Span
	Statements  []*statement
	Declaration *declaration
	Expression  *expression
	Initializer *statement
	Condition   *expression
	Post        *expression
	Then        *statement
	Else        *statement
	Cases       []switchCase
	Iterators   []parameter
}

type switchCase struct {
	Default    bool
	Expression *expression
	Statements []*statement
	Span       Span
}

type expressionKind uint8

const (
	expressionIdentifier expressionKind = iota
	expressionLiteral
	expressionUnary
	expressionBinary
	expressionCall
	expressionMember
	expressionIndex
	expressionNew
	expressionArray
	expressionTernary
	expressionCast
)

type expression struct {
	Kind     expressionKind
	Text     string
	Type     typeRef
	Left     *expression
	Right    *expression
	Third    *expression
	Receiver *expression
	Args     []*expression
	Span     Span
}

type program struct {
	Declarations []*declaration
}
