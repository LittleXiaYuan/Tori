import { redirect } from "next/navigation";

export default function LegacyBackupRedirectPage() {
  redirect("/packs/backup");
}
