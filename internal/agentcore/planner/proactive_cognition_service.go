package planner

// ProactiveCognitionService owns Planner's proactive cognition surface.
//
// It intentionally keeps Reverie and event producers together while staying out
// of the canonical post-turn reflection path, which belongs to cognikernel.
// Planner asks this service for optional journal context and reports execution
// outcomes that may become proactive events.
type ProactiveCognitionService struct {
	reverie        *Reverie
	failureMonitor *TaskFailureMonitor
}

func NewProactiveCognitionService() *ProactiveCognitionService {
	return &ProactiveCognitionService{}
}

func (s *ProactiveCognitionService) SetReverie(reverie *Reverie) {
	if s == nil {
		return
	}
	s.reverie = reverie
}

func (s *ProactiveCognitionService) Reverie() *Reverie {
	if s == nil {
		return nil
	}
	return s.reverie
}

func (s *ProactiveCognitionService) SetTaskFailureMonitor(monitor *TaskFailureMonitor) {
	if s == nil {
		return
	}
	s.failureMonitor = monitor
}

func (s *ProactiveCognitionService) RecordExecutionFailure(failed bool) bool {
	if s == nil || s.failureMonitor == nil {
		return false
	}
	return s.failureMonitor.Record(failed)
}

func (s *ProactiveCognitionService) JournalContext(maxThoughts int, query string) string {
	if s == nil || s.reverie == nil {
		return ""
	}
	return s.reverie.JournalContext(maxThoughts, query)
}
