// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// https://astro.build/config
export default defineConfig({
	site: 'https://mitchellnemitz.github.io',
	base: '/wisp',
	integrations: [
		starlight({
			title: 'wisp',
			social: [
				{ icon: 'github', label: 'GitHub', href: 'https://github.com/mitchellnemitz/wisp' },
			],
			// sidebar added in Task 3, expressiveCode in Task 2, plugins in Task 4
		}),
	],
});
