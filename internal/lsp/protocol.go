package lsp

// LSP wire types (the subset this server uses). All positions are 0-based with
// the character field counted in UTF-16 code units, per the LSP spec.

type lspPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type lspRange struct {
	Start lspPosition `json:"start"`
	End   lspPosition `json:"end"`
}

// Diagnostic severities.
const (
	severityError   = 1
	severityWarning = 2
)

type lspDiagnostic struct {
	Range    lspRange `json:"range"`
	Severity int      `json:"severity"`
	Source   string   `json:"source,omitempty"`
	Message  string   `json:"message"`
}

type publishDiagnosticsParams struct {
	URI         string          `json:"uri"`
	Diagnostics []lspDiagnostic `json:"diagnostics"`
}

type textEdit struct {
	Range   lspRange `json:"range"`
	NewText string   `json:"newText"`
}

// SymbolKind values (LSP spec).
const (
	symbolKindEnum          = 10
	symbolKindFunction      = 12
	symbolKindStruct        = 23
	symbolKindTypeParameter = 26 // used for transparent type aliases
)

type documentSymbol struct {
	Name           string           `json:"name"`
	Kind           int              `json:"kind"`
	Range          lspRange         `json:"range"`
	SelectionRange lspRange         `json:"selectionRange"`
	Children       []documentSymbol `json:"children,omitempty"`
}

type markupContent struct {
	Kind  string `json:"kind"` // "plaintext"
	Value string `json:"value"`
}

type hoverResult struct {
	Contents markupContent `json:"contents"`
	Range    *lspRange     `json:"range,omitempty"`
}

// CompletionItemKind values (LSP spec).
const (
	completionKindFunction = 3
	completionKindVariable = 6
	completionKindClass    = 7
	completionKindEnum     = 13
	completionKindKeyword  = 14
	completionKindConstant = 21
)

type completionItem struct {
	Label  string `json:"label"`
	Kind   int    `json:"kind"`
	Detail string `json:"detail,omitempty"`
}

// --- request param shapes ---

type textDocumentItem struct {
	URI  string `json:"uri"`
	Text string `json:"text"`
}

type textDocumentIdentifier struct {
	URI string `json:"uri"`
}

type didOpenParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

type contentChange struct {
	Text string `json:"text"`
}

type didChangeParams struct {
	TextDocument   textDocumentIdentifier `json:"textDocument"`
	ContentChanges []contentChange        `json:"contentChanges"`
}

type didCloseParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

type docRequestParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     lspPosition            `json:"position"`
}

type location struct {
	URI   string   `json:"uri"`
	Range lspRange `json:"range"`
}

type referenceParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     lspPosition            `json:"position"`
	Context      struct {
		IncludeDeclaration bool `json:"includeDeclaration"`
	} `json:"context"`
}

type renameParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     lspPosition            `json:"position"`
	NewName      string                 `json:"newName"`
}

type workspaceEdit struct {
	Changes map[string][]textEdit `json:"changes"`
}

type signatureInformation struct {
	Label      string                 `json:"label"`
	Parameters []parameterInformation `json:"parameters,omitempty"`
}

type parameterInformation struct {
	Label string `json:"label"`
}

type signatureHelp struct {
	Signatures      []signatureInformation `json:"signatures"`
	ActiveSignature int                    `json:"activeSignature"`
	ActiveParameter int                    `json:"activeParameter"`
}

type codeActionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Range        lspRange               `json:"range"`
	Context      struct {
		Diagnostics []lspDiagnostic `json:"diagnostics"`
	} `json:"context"`
}

type codeAction struct {
	Title       string          `json:"title"`
	Kind        string          `json:"kind"`
	Diagnostics []lspDiagnostic `json:"diagnostics,omitempty"`
	Edit        workspaceEdit   `json:"edit"`
}
