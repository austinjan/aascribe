use crate::cli::Command;
use crate::error::{AppError, AppResult};
use crate::output::CommandResult;

pub fn execute(command: &Command, store_path: &std::path::Path) -> AppResult<CommandResult> {
    match command {
        Command::Init(args) => init::run(store_path, args.force),
        Command::Index(_) => not_implemented("index"),
        Command::Describe(_) => not_implemented("describe"),
        Command::Remember(_) => not_implemented("remember"),
        Command::Consolidate(_) => not_implemented("consolidate"),
        Command::Recall(_) => not_implemented("recall"),
        Command::List(_) => not_implemented("list"),
        Command::Show(_) => not_implemented("show"),
        Command::Forget(_) => not_implemented("forget"),
    }
}

fn not_implemented(command: &'static str) -> AppResult<CommandResult> {
    Err(AppError::NotImplemented { command })
}

mod init {
    use std::path::Path;

    use serde_json::json;

    use crate::error::AppResult;
    use crate::output::CommandResult;
    use crate::store;

    pub fn run(store_path: &Path, force: bool) -> AppResult<CommandResult> {
        let outcome = store::initialize_store(store_path, force)?;
        let text = if outcome.reinitialized {
            format!("Reinitialized aascribe store at {}", store_path.display())
        } else {
            format!("Initialized aascribe store at {}", store_path.display())
        };

        Ok(CommandResult::new(
            json!({
                "store": store_path.to_string_lossy().to_string(),
                "created": outcome.created,
                "reinitialized": outcome.reinitialized,
                "layout_version": store::layout_version(),
            }),
            text,
        ))
    }
}
