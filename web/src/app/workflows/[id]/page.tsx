import WorkflowEditorPage from "./workflow-editor";

export async function generateStaticParams() {
  return [{ id: "_" }];
}

export default function Page() {
  return <WorkflowEditorPage />;
}
