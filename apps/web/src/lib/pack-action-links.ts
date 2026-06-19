export function chatPromptHref(prompt: string): string {
  return `/chat?q=${encodeURIComponent(prompt.trim())}`;
}

export function taskDetailHref(taskId: string): string {
  return `/task-detail?id=${encodeURIComponent(taskId.trim())}`;
}

export function traceTaskHref(taskId: string): string {
  return `/trace?task=${encodeURIComponent(taskId.trim())}`;
}
