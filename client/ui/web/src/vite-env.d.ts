/// <reference types="svelte" />
/// <reference types="vite/client" />

declare module "@docs/*.md?raw" {
  const content: string;
  export default content;
}
