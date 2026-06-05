package framework

type Directive int

const (
	Restart Directive = iota
	Stop
	Escalate
)

func (d Directive) String() string {
	switch d {
	case Restart:
		return "Restart"
	case Stop:
		return "Stop"
	case Escalate:
		return "Escalate"
	default:
		return "Unknown"
	}
}

type SupervisionAlert struct {
	Child  ActorRef
	Reason any
}

type SupervisorStrategy interface {
	Decide(alert SupervisionAlert) Directive
}

type StrategyFunc func(SupervisionAlert) Directive

func (f StrategyFunc) Decide(alert SupervisionAlert) Directive { return f(alert) }

func DefaultStrategy() SupervisorStrategy {
	return StrategyFunc(func(SupervisionAlert) Directive { return Restart })
}

func AlwaysStop() SupervisorStrategy {
	return StrategyFunc(func(SupervisionAlert) Directive { return Stop })
}

func AlwaysEscalate() SupervisorStrategy {
	return StrategyFunc(func(SupervisionAlert) Directive { return Escalate })
}

type escalation struct {
	child  ActorRef
	reason any
}
