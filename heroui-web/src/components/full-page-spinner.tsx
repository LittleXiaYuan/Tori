import { Spinner } from "@heroui/react";

export default function FullPageSpinner() {
  return (
    <div className="flex items-center justify-center h-[60vh]">
      <Spinner size="lg" />
    </div>
  );
}
