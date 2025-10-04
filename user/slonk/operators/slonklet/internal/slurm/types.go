package slurm

const (
	SLURMRESTD_SOCKET = "/etc/slurm/slurmrestd/slurmrestd.sock"
)

type SlurmResponse struct {
	Nodes        []SlurmNode   `json:"nodes,omitempty"`
	LastBackfill *BackfillType `json:"last_backfill,omitempty"`
	Meta         *MetaType     `json:"meta,omitempty"`
	Jobs         []SlurmJob    `json:"jobs,omitempty"`
	LastUpdate   *FlagType     `json:"last_update,omitempty"`
	Warnings     []Warning     `json:"warnings,omitempty"`
	Errors       []ErrorType   `json:"errors,omitempty"`
}

type SlurmNode struct {
	Name string

	Architecture string   `json:"architecture"`
	Features     []string `json:"features"`
	State        []string `json:"state"`
	Reason       string   `json:"reason"`
	Comment      string   `json:"comment"`
	Reservation  string   `json:"reservation"`

	Gres        string `json:"gres"`
	GresDrained string `json:"gres_drained"`
	GresUsed    string `json:"gres_used"`

	Tres         string  `json:"tres"`
	TresUsed     string  `json:"tres_used"`
	TresWeighted float32 `json:"tres_weighted"`

	AllocCpus     int `json:"alloc_cpus"`
	AllocIdleCpus int `json:"alloc_idle_cpus"`
	AllocMemory   int `json:"alloc_mem"`

	BootTime        FlagType `json:"boot_time"`
	SlurmdStartTime FlagType `json:"slurmd_start_time"`
	LastBusy        FlagType `json:"last_busy"`
}

type SlurmJob struct {
	JobID int    `json:"job_id"`
	Name  string `json:"name"`

	Command                 string `json:"command"`
	Comment                 string `json:"comment"`
	Container               string `json:"container,omitempty"`
	UserID                  int    `json:"user_id,omitempty"`
	UserName                string `json:"user_name,omitempty"`
	CurrentWorkingDirectory string `json:"current_working_directory,omitempty"`
	StandardError           string `json:"standard_error,omitempty"`
	StandardOutput          string `json:"standard_output,omitempty"`

	SubmitTime  FlagType `json:"submit_time"`
	SuspendTime FlagType `json:"suspend_time"`
	StartTime   FlagType `json:"start_time,omitempty"`
	EndTime     FlagType `json:"end_time,omitempty"`
	PreemptTime FlagType `json:"preempt_time,omitempty"`
	TimeLimit   FlagType `json:"time_limit,omitempty"`
	TimeMinimum FlagType `json:"time_minimum,omitempty"`

	JobState     string   `json:"job_state,omitempty"`
	StateReason  string   `json:"state_reason,omitempty"`
	Priority     FlagType `json:"priority,omitempty"`
	RestartCount int      `json:"restart_cnt,omitempty"`

	Cluster       string       `json:"cluster,omitempty"`
	MemoryPerTRES string       `json:"memory_per_tres,omitempty"`
	NodeCount     FlagType     `json:"node_count,omitempty"`
	Partition     string       `json:"partition,omitempty"`
	ResvName      string       `json:"resv_name,omitempty"`
	Nodes         string       `json:"nodes,omitempty"`
	JobResources  JobResources `json:"job_resources,omitempty"`
}

type JobResources struct {
	AllocatedNodes []AllocatedNode `json:"allocated_nodes,omitempty"`
}

type AllocatedNode struct {
	NodeName string `json:"nodename,omitempty"`
}

type FlagType struct {
	Number   int  `json:"number,omitempty"`
	Set      bool `json:"set,omitempty"`
	Infinite bool `json:"infinite,omitempty"`
}

type BackfillType struct {
	Number   int  `json:"number,omitempty"`
	Set      bool `json:"set,omitempty"`
	Infinite bool `json:"infinite,omitempty"`
}

type MetaType struct {
	Slurm   SlurmType  `json:"slurm,omitempty"`
	Plugin  PluginType `json:"plugin,omitempty"`
	Client  ClientType `json:"client,omitempty"`
	Command []string   `json:"command,omitempty"`
}

type Warning struct {
	Description string `json:"description,omitempty"`
	Source      string `json:"source,omitempty"`
}

type ErrorType struct {
	Description string `json:"description,omitempty"`
	Source      string `json:"source,omitempty"`
	Error       string `json:"error,omitempty"`
	ErrorNumber int    `json:"error_number,omitempty"`
}

type SlurmType struct {
	Cluster string      `json:"cluster,omitempty"`
	Release string      `json:"release,omitempty"`
	Version VersionType `json:"version,omitempty"`
}

type VersionType struct {
	Major int `json:"major,omitempty"`
	Minor int `json:"minor,omitempty"`
	Micro int `json:"micro,omitempty"`
}

type PluginType struct {
	AccountingStorage string `json:"accounting_storage,omitempty"`
	Name              string `json:"name,omitempty"`
	Type              string `json:"type,omitempty"`
	DataParser        string `json:"data_parser,omitempty"`
}

type ClientType struct {
	Source string `json:"source,omitempty"`
	User   string `json:"user,omitempty"`
	Group  string `json:"group,omitempty"`
}
