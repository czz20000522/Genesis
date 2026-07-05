import MarkdownIt from 'markdown-it'

const renderer = new MarkdownIt({
  breaks: true,
  html: false,
  linkify: true,
  typographer: false,
})

renderer.validateLink = (url) => /^https?:\/\//i.test(url)

const defaultFence = renderer.renderer.rules.fence

renderer.renderer.rules.fence = (tokens, idx, options, env, self) => {
  const token = tokens[idx]
  const language = String(token.info || '').trim().split(/\s+/)[0]?.toLowerCase()
  if (language !== 'mermaid') {
    return defaultFence ? defaultFence(tokens, idx, options, env, self) : self.renderToken(tokens, idx, options)
  }
  const code = String(token.content || '')
  return `<section class="mermaid-card" data-mermaid-code="${escapeHtml(encodeURIComponent(code))}">
    <div class="mermaid-toolbar">
      <div class="mermaid-tabs" role="tablist" aria-label="图表视图">
        <button type="button" class="mermaid-tab mermaid-tab-active" data-mermaid-action="show-diagram">图表</button>
        <button type="button" class="mermaid-tab" data-mermaid-action="show-code">代码</button>
      </div>
      <div class="mermaid-actions">
        <button type="button" data-mermaid-action="copy-code">复制代码</button>
        <button type="button" data-mermaid-action="copy-svg">复制图片</button>
        <button type="button" data-mermaid-action="download-svg">下载</button>
      </div>
    </div>
    <div class="mermaid-panel mermaid-panel-diagram" data-mermaid-panel="diagram">正在渲染图表...</div>
    <pre class="mermaid-panel mermaid-panel-code" data-mermaid-panel="code" hidden><code>${escapeHtml(code)}</code></pre>
  </section>`
}

export function assistantMarkdown(markdown: string) {
  return renderer.render(String(markdown || ''))
}

function escapeHtml(value: string) {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}
