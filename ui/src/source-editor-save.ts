export interface SourceEditorRevision {
  projectId: string;
  filePath: string;
  generation: number;
  content: string;
  original: string;
}

export interface SourceSaveRequest {
  projectId: string;
  filePath: string;
  generation: number;
  content: string;
}

export function completeSourceSave(
  current: SourceEditorRevision,
  saved: SourceSaveRequest,
): SourceEditorRevision {
  if (
    current.projectId !== saved.projectId ||
    current.filePath !== saved.filePath ||
    current.generation !== saved.generation
  ) return current;

  return { ...current, original: saved.content };
}
