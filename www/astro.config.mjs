// @ts-check
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// Resolve the wisp grammar relative to this config file, not the process cwd,
// so the build works regardless of the directory it is invoked from.
const wispGrammarPath = fileURLToPath(
	new URL('../editors/vscode/syntaxes/wisp.tmLanguage.json', import.meta.url),
);

// https://astro.build/config
export default defineConfig({
	site: 'https://mitchellnemitz.github.io',
	base: '/wisp',
	integrations: [
		starlight({
			title: 'wisp',
			expressiveCode: {
				shiki: {
					langs: [
						JSON.parse(fs.readFileSync(wispGrammarPath, 'utf-8')),
					],
				},
			},
			social: [
				{ icon: 'github', label: 'GitHub', href: 'https://github.com/mitchellnemitz/wisp' },
			],
			// sidebar added in Task 3, plugins in Task 4
		}),
	],
});
