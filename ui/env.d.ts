/// <reference types="vite/client" />

// Single-file components are compiled by @vitejs/plugin-vue; give TypeScript a
// module shim so imports of `*.vue` resolve during type-checking.
declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<Record<string, unknown>, Record<string, unknown>, unknown>
  export default component
}

// Injected by Vite's `define` (see vite.config.ts).
declare const __UI_BUILD__: { git_sha: string; build_time: string }
