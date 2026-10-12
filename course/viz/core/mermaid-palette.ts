// Mermaid не читает CSS-переменные, поэтому палитра продублирована здесь.
// Смысл цветов совпадает с viz/palette.css: клиент синий, сервис фиолетовый, БД зелёная…
export type ThemeName = 'dark' | 'light'

export const MERMAID_CLASSES = ['client', 'service', 'db', 'cache', 'broker', 'network', 'deep'] as const
export type MermaidClass = (typeof MERMAID_CLASSES)[number]

interface ClassStyle {
  fill: string
  stroke: string
  color: string
  dashed?: boolean
}

const CLASS_STYLES: Record<ThemeName, Record<MermaidClass, ClassStyle>> = {
  dark: {
    client: { fill: '#0c1a33', stroke: '#60a5fa', color: '#dbeafe' },
    service: { fill: '#1c1433', stroke: '#a78bfa', color: '#ede9fe' },
    db: { fill: '#062a20', stroke: '#34d399', color: '#d1fae5' },
    cache: { fill: '#2a1d05', stroke: '#fbbf24', color: '#fef3c7' },
    broker: { fill: '#062a30', stroke: '#22d3ee', color: '#cffafe' },
    network: { fill: '#111b2e', stroke: '#94a3b8', color: '#e2e8f0' },
    deep: { fill: '#0f1b2e', stroke: '#38bdf8', color: '#e0f2fe', dashed: true },
  },
  light: {
    client: { fill: '#eff6ff', stroke: '#2563eb', color: '#1e3a8a' },
    service: { fill: '#f5f3ff', stroke: '#7c3aed', color: '#4c1d95' },
    db: { fill: '#ecfdf5', stroke: '#059669', color: '#064e3b' },
    cache: { fill: '#fffbeb', stroke: '#d97706', color: '#78350f' },
    broker: { fill: '#ecfeff', stroke: '#0891b2', color: '#164e63' },
    network: { fill: '#f8fafc', stroke: '#64748b', color: '#1e293b' },
    deep: { fill: '#f0f9ff', stroke: '#0284c7', color: '#0c4a6e', dashed: true },
  },
}

const THEME_VARIABLES: Record<ThemeName, Record<string, string>> = {
  dark: {
    background: '#0b1220',
    primaryColor: '#111b2e',
    primaryTextColor: '#e2e8f0',
    primaryBorderColor: '#38bdf8',
    secondaryColor: '#0f172a',
    tertiaryColor: '#0f172a',
    lineColor: '#64748b',
    textColor: '#e2e8f0',
    clusterBkg: '#0f172a',
    clusterBorder: '#1e293b',
    edgeLabelBackground: '#0b1220',
    fontFamily: "'Inter Variable', Inter, sans-serif",
  },
  light: {
    background: '#ffffff',
    primaryColor: '#f8fafc',
    primaryTextColor: '#0f172a',
    primaryBorderColor: '#0284c7',
    secondaryColor: '#f1f5f9',
    tertiaryColor: '#ffffff',
    lineColor: '#64748b',
    textColor: '#0f172a',
    clusterBkg: '#f8fafc',
    clusterBorder: '#e2e8f0',
    edgeLabelBackground: '#ffffff',
    fontFamily: "'Inter Variable', Inter, sans-serif",
  },
}

export function mermaidThemeVariables(theme: ThemeName): Record<string, string> {
  return { ...THEME_VARIABLES[theme] }
}

function classDef(name: MermaidClass, s: ClassStyle): string {
  const dash = s.dashed ? ',stroke-dasharray:5 3' : ''
  return `  classDef ${name} fill:${s.fill},stroke:${s.stroke},color:${s.color},stroke-width:2px${dash}`
}

const FLOWCHART_HEADER = /^\s*(flowchart|graph)\b/

export function withPaletteClasses(code: string, theme: ThemeName): string {
  const lines = code.split('\n')
  const header = lines.findIndex((line) => line.trim() !== '')
  if (header === -1 || !FLOWCHART_HEADER.test(lines[header] ?? '')) return code
  const defs = MERMAID_CLASSES.map((name) => classDef(name, CLASS_STYLES[theme][name]))
  return [...lines.slice(0, header + 1), ...defs, ...lines.slice(header + 1)].join('\n')
}
