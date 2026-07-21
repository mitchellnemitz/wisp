// @ts-check
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import { unified } from '@astrojs/markdown-remark';
import { visit } from 'unist-util-visit';
import starlightLinksValidator from 'starlight-links-validator';

// Resolve the wisp grammar relative to this config file, not the process cwd,
// so the build works regardless of the directory it is invoked from.
const wispGrammarPath = fileURLToPath(
	new URL('../editors/vscode/syntaxes/wisp.tmLanguage.json', import.meta.url),
);

const base = '/wisp';

// Starlight's own nav/sidebar/pagination links are base-prefixed automatically,
// but plain markdown content links are rendered verbatim by the underlying
// remark/rehype pipeline and are never rewritten for `base`. Content is written
// with root-relative, base-agnostic paths (e.g. `/guide/testing/`), so a small
// rehype plugin prepends `base` to those hrefs at build time - keeping the
// content itself free of any hardcoded `/wisp` prefix.
/** @type {import('unified').Plugin<[], import('hast').Root>} */
function rehypePrependBase() {
	return (tree) => {
		visit(tree, 'element', (node) => {
			if (node.tagName !== 'a') return;
			const href = node.properties?.href;
			if (typeof href !== 'string') return;
			if (!href.startsWith('/') || href.startsWith('//') || href.startsWith(base + '/') || href === base) {
				return;
			}
			node.properties.href = base + href;
		});
	};
}

// https://astro.build/config
export default defineConfig({
	site: 'https://mitchellnemitz.github.io',
	base,
	markdown: {
		processor: unified({ rehypePlugins: [rehypePrependBase] }),
	},
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
			sidebar: [
				{ label: 'Guide', items: [{ autogenerate: { directory: 'guide' } }] },
				{ label: 'Reference', items: [
					{ slug: 'stdlib-index' },
					{ slug: 'design-decisions' },
				] },
			],
			plugins: [
				starlightLinksValidator({
					errorOnInvalidHashes: false, // FR-010: #anchor fragments out of scope
				}),
			],
		}),
	],
});
