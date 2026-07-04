import MarkdownIt from 'markdown-it'

const renderer = new MarkdownIt({
  breaks: true,
  html: false,
  linkify: true,
  typographer: false,
})

renderer.validateLink = (url) => /^https?:\/\//i.test(url)

export function assistantMarkdown(markdown: string) {
  return renderer.render(String(markdown || ''))
}
