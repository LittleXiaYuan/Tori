import BotDetailClient from "./bot-detail";

export async function generateStaticParams() {
  return [{ id: "_" }];
}

export default function BotDetailPage() {
  return <BotDetailClient />;
}
