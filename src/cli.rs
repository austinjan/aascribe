use clap::{Args, Parser, Subcommand, ValueEnum};

#[derive(Debug, Parser)]
#[command(
    name = "aascribe",
    version,
    about = "Memory-first local CLI for project recall"
)]
pub struct Cli {
    #[arg(long, global = true)]
    pub store: Option<String>,

    #[arg(long, global = true, value_enum, default_value_t = Format::Json)]
    pub format: Format,

    #[arg(long, short = 'q', global = true)]
    pub quiet: bool,

    #[arg(long, short = 'v', global = true)]
    pub verbose: bool,

    #[command(subcommand)]
    pub command: Command,
}

#[derive(Clone, Copy, Debug, Default, Eq, PartialEq, ValueEnum)]
pub enum Format {
    #[default]
    Json,
    Text,
}

#[derive(Debug, Subcommand)]
pub enum Command {
    Init(InitArgs),
    Index(IndexArgs),
    Describe(DescribeArgs),
    Remember(RememberArgs),
    Consolidate(ConsolidateArgs),
    Recall(RecallArgs),
    List(ListArgs),
    Show(ShowArgs),
    Forget(ForgetArgs),
}

impl Command {
    pub fn name(&self) -> &'static str {
        match self {
            Self::Init(_) => "init",
            Self::Index(_) => "index",
            Self::Describe(_) => "describe",
            Self::Remember(_) => "remember",
            Self::Consolidate(_) => "consolidate",
            Self::Recall(_) => "recall",
            Self::List(_) => "list",
            Self::Show(_) => "show",
            Self::Forget(_) => "forget",
        }
    }
}

#[derive(Debug, Args)]
pub struct InitArgs {
    #[arg(long)]
    pub force: bool,
}

#[derive(Debug, Args)]
pub struct IndexArgs {
    pub path: String,

    #[arg(long, default_value_t = 3)]
    pub depth: i32,

    #[arg(long)]
    pub include: Vec<String>,

    #[arg(long)]
    pub exclude: Vec<String>,

    #[arg(long)]
    pub refresh: bool,

    #[arg(long)]
    pub no_summary: bool,

    #[arg(long, default_value_t = 1_048_576)]
    pub max_file_size: u64,
}

#[derive(Debug, Args)]
pub struct DescribeArgs {
    pub file: String,

    #[arg(long)]
    pub refresh: bool,

    #[arg(long, value_enum, default_value_t = SummaryLength::Medium)]
    pub length: SummaryLength,

    #[arg(long)]
    pub focus: Option<String>,
}

#[derive(Clone, Copy, Debug, Default, Eq, PartialEq, ValueEnum)]
pub enum SummaryLength {
    Short,
    #[default]
    Medium,
    Long,
}

#[derive(Debug, Args)]
pub struct RememberArgs {
    pub content: Option<String>,

    #[arg(long)]
    pub tag: Vec<String>,

    #[arg(long)]
    pub source: Option<String>,

    #[arg(long)]
    pub session: Option<String>,

    #[arg(long)]
    pub ttl: Option<String>,

    #[arg(long, default_value_t = 3)]
    pub importance: u8,

    #[arg(long)]
    pub stdin: bool,
}

#[derive(Debug, Args)]
pub struct ConsolidateArgs {
    #[arg(long, default_value = "7d")]
    pub since: String,

    #[arg(long, default_value = "now")]
    pub until: String,

    #[arg(long)]
    pub session: Option<String>,

    #[arg(long)]
    pub tag: Vec<String>,

    #[arg(long)]
    pub topic: Option<String>,

    #[arg(long)]
    pub dry_run: bool,

    #[arg(long)]
    pub keep_short: bool,

    #[arg(long, default_value_t = 3)]
    pub min_items: usize,
}

#[derive(Debug, Args)]
pub struct RecallArgs {
    pub query: String,

    #[arg(long, value_enum, default_value_t = MemoryTier::All)]
    pub tier: MemoryTier,

    #[arg(long, default_value_t = 10)]
    pub limit: usize,

    #[arg(long)]
    pub tag: Vec<String>,

    #[arg(long)]
    pub since: Option<String>,

    #[arg(long)]
    pub until: Option<String>,

    #[arg(long, default_value_t = 0.3)]
    pub min_score: f32,

    #[arg(long)]
    pub include_source: bool,
}

#[derive(Clone, Copy, Debug, Default, Eq, PartialEq, ValueEnum)]
pub enum MemoryTier {
    Short,
    Long,
    #[default]
    All,
}

#[derive(Debug, Args, Default)]
pub struct ListArgs {
    #[arg(long, value_enum, default_value_t = MemoryTier::All)]
    pub tier: MemoryTier,

    #[arg(long)]
    pub session: Option<String>,

    #[arg(long)]
    pub tag: Vec<String>,

    #[arg(long)]
    pub since: Option<String>,

    #[arg(long)]
    pub until: Option<String>,

    #[arg(long, default_value_t = 50)]
    pub limit: usize,

    #[arg(long, value_enum, default_value_t = SortOrder::Desc)]
    pub order: SortOrder,
}

#[derive(Clone, Copy, Debug, Default, Eq, PartialEq, ValueEnum)]
pub enum SortOrder {
    Asc,
    #[default]
    Desc,
}

#[derive(Debug, Args)]
pub struct ShowArgs {
    pub id: String,
}

#[derive(Debug, Args)]
pub struct ForgetArgs {
    pub id: String,

    #[arg(long)]
    pub force: bool,
}
