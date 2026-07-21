// @ts-check
import fs from 'node:fs';
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

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
						JSON.parse(
							fs.readFileSync('../editors/vscode/syntaxes/wisp.tmLanguage.json', 'utf-8'),
						),
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
