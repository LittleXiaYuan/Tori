package gateway

//  from handlers_triggers.go
// Trigger HTTP handlers (legacy Runtime + unified Manager) were de-shelled into
// the triggers pack (internal/packs/triggers); the subsystems are reached there
// via the gateway's TriggerRuntime()/TriggerManager() accessors.

//  from handlers_triggers_v2.go
// 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
// Triggers V2 API 鈥?TriggerDef + TriggerRun + TriggerEvent
//
// GET    /v1/triggers/v2           鈥?list all triggers (filterable)
// GET    /v1/triggers/v2?id=xxx    鈥?get one trigger
// POST   /v1/triggers/v2           鈥?create trigger (TriggerDef)
// PUT    /v1/triggers/v2           鈥?update trigger
// DELETE /v1/triggers/v2?id=xxx    鈥?delete trigger
//
// POST   /v1/triggers/v2/emit      鈥?emit event
// GET    /v1/triggers/v2/runs      鈥?list runs
// GET    /v1/triggers/v2/events    鈥?list events
// 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

// Cron HTTP handlers were de-shelled into the cron pack (internal/packs/cron);
// the cron manager is reached there via the gateway's CronManager() accessor.

// Session queue HTTP handlers were de-shelled into the session-queue pack
// (internal/packs/sessionqueue). The gateway only exposes QueueManager().
