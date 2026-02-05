import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
	site: 'https://albertocavalcante.github.io',
	base: '/lspls',

	integrations: [
		starlight({
			title: 'lspls',
			description: 'Generate Go types from the LSP specification',
			favicon: '/favicon.svg',
			lastUpdated: true,
			tableOfContents: { minHeadingLevel: 2, maxHeadingLevel: 3 },
			expressiveCode: {
				themes: ['github-dark', 'github-light'],
				styleOverrides: {
					borderRadius: '0.625rem',
					codeFontFamily: "'JetBrains Mono', 'SF Mono', 'Fira Code', ui-monospace, monospace",
					codeFontSize: '0.875rem',
					codeLineHeight: '1.65',
				},
			},
			social: [
				{ icon: 'github', label: 'GitHub', href: 'https://github.com/albertocavalcante/lspls' },
			],
			editLink: {
				baseUrl: 'https://github.com/albertocavalcante/lspls/edit/main/docs/',
			},
			customCss: ['./src/styles/global.css'],
			sidebar: [
				{
					label: 'Getting Started',
					items: [
						{ label: 'Introduction', slug: 'getting-started/introduction' },
						{ label: 'Installation', slug: 'getting-started/installation' },
						{ label: 'Quick Start', slug: 'getting-started/quick-start' },
					],
				},
				{
					label: 'Reference',
					items: [
						{ label: 'CLI Reference', slug: 'reference/cli' },
						{ label: 'Generated Code', slug: 'reference/generated-code' },
						{ label: 'LSP Versions', slug: 'reference/lsp-versions' },
					],
				},
				{
					label: 'Resources',
					items: [
						{ label: 'Further Reading', slug: 'resources/further-reading' },
					],
				},
			],
		}),
	],
});
