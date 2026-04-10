package opp

const Version = 3

type MessageType string

const (
	// Protocol level
	MsgHello   MessageType = "HELLO"
	MsgWelcome MessageType = "WELCOME"
	MsgBye     MessageType = "BYE"
	MsgPing    MessageType = "PING"
	MsgPong    MessageType = "PONG"
	MsgAck     MessageType = "ACK"
	MsgError   MessageType = "ERROR"
	MsgCancel  MessageType = "CANCEL"
	MsgResume  MessageType = "RESUME"

	// Business level
	MsgIntent      MessageType = "INTENT"
	MsgAccept      MessageType = "ACCEPT"
	MsgReject      MessageType = "REJECT"
	MsgResult      MessageType = "RESULT"
	MsgQuestion    MessageType = "QUESTION"
	MsgAnswer      MessageType = "ANSWER"
	MsgProgress    MessageType = "PROGRESS"
	MsgProblem     MessageType = "PROBLEM"
	MsgDecide      MessageType = "DECIDE"
	MsgObservation MessageType = "OBSERVATION"
	MsgActionTaken MessageType = "ACTION_TAKEN"
	MsgHeartbeat   MessageType = "HEARTBEAT"
	MsgFeedback    MessageType = "FEEDBACK"

	// Agent network level (v3)
	MsgNotify         MessageType = "NOTIFY"
	MsgSubscribe      MessageType = "SUBSCRIBE"
	MsgUnsubscribe    MessageType = "UNSUBSCRIBE"
	MsgEvent          MessageType = "EVENT"
	MsgDelegate       MessageType = "DELEGATE"
	MsgDelegateResult MessageType = "DELEGATE_RESULT"
	MsgDiscover       MessageType = "DISCOVER"
	MsgCapabilities   MessageType = "CAPABILITIES"
)

type TaskState string

const (
	StatePending      TaskState = "pending"
	StateAccepted     TaskState = "accepted"
	StateRunning      TaskState = "running"
	StateWaitingInput TaskState = "waiting_input"
	StateBlocked      TaskState = "blocked"
	StateCompleted    TaskState = "completed"
	StateFailed       TaskState = "failed"
	StateCancelled    TaskState = "cancelled"
	StateTimedOut     TaskState = "timed_out"
)

type ErrorCode string

const (
	ErrCodeSessionNotFound   ErrorCode = "SESSION_NOT_FOUND"
	ErrCodeTaskNotFound      ErrorCode = "TASK_NOT_FOUND"
	ErrCodeUnknownIntent     ErrorCode = "UNKNOWN_INTENT"
	ErrCodePayloadInvalid    ErrorCode = "PAYLOAD_INVALID"
	ErrCodePermissionDenied  ErrorCode = "PERMISSION_DENIED"
	ErrCodeAgentOffline      ErrorCode = "AGENT_OFFLINE"
	ErrCodeRateLimited       ErrorCode = "RATE_LIMITED"
	ErrCodeInternalError     ErrorCode = "INTERNAL_ERROR"
	ErrCodeVersionMismatch   ErrorCode = "VERSION_MISMATCH"
	ErrCodeInvalidTransition ErrorCode = "INVALID_TRANSITION"
)
