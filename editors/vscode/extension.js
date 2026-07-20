// Entry point for the wisp VSCode extension. The TextMate grammar and language
// configuration provide highlighting with no activation; this code adds the
// language server (diagnostics, formatting, symbols, hover, completion) by
// launching `wisp-lsp` and connecting a generic LSP client to it over stdio.
//
// Plain CommonJS so the extension needs no compile step: `npm install` then
// `vsce package` bundles vscode-languageclient into the .vsix.

const { workspace, window } = require("vscode");
const { LanguageClient, TransportKind } = require("vscode-languageclient/node");

/** @type {import('vscode-languageclient/node').LanguageClient | undefined} */
let client;

function activate() {
  const config = workspace.getConfiguration("wisp");
  if (config.get("lsp.enable") === false) {
    return;
  }

  const command = config.get("lsp.path") || "wisp-lsp";
  const serverOptions = {
    run: { command, transport: TransportKind.stdio },
    debug: { command, transport: TransportKind.stdio },
  };
  const clientOptions = {
    documentSelector: [{ scheme: "file", language: "wisp" }],
  };

  client = new LanguageClient(
    "wisp-lsp",
    "wisp Language Server",
    serverOptions,
    clientOptions
  );
  client.start().catch((err) => {
    // A missing or unstartable server must not break highlighting; surface it
    // and carry on.
    window.showWarningMessage(
      `wisp: could not start the language server (${command}). ` +
        `Highlighting still works. Set "wisp.lsp.path" or disable with ` +
        `"wisp.lsp.enable". Details: ${err && err.message ? err.message : err}`
    );
  });
}

function deactivate() {
  if (client) {
    return client.stop();
  }
  return undefined;
}

module.exports = { activate, deactivate };
