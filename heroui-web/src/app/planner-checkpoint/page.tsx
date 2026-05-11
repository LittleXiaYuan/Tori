"use client";

import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { PlannerCheckpointDetail } from "@/components/planner/planner-checkpoint-detail";

function PlannerCheckpointPageContent() {
  const params = useSearchParams();
  return <PlannerCheckpointDetail planId={params.get("plan_id") || ""} initialResumePlanJobId={params.get("job_id") || ""} />;
}

export default function PlannerCheckpointPage() {
  return (
    <Suspense fallback={null}>
      <PlannerCheckpointPageContent />
    </Suspense>
  );
}
