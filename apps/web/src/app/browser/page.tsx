import { redirect } from "next/navigation";

export default function LegacyBrowserRedirectPage() {
  redirect("/packs/browser");
}
