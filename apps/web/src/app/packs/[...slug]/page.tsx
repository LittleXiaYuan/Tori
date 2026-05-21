import PackRuntimeRouteClientPage from "./client-page";

export const dynamic = "force-static";

export function generateStaticParams() {
  return [
    { slug: ["_pack-runtime-route-shell"] },
  ];
}

export default function PackRuntimeRoutePage() {
  return <PackRuntimeRouteClientPage />;
}
