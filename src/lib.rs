mod cli;
mod command;
mod error;
mod output;
mod store;

use std::process::ExitCode;
use std::time::Instant;

use clap::Parser;

use cli::Cli;
use error::{AppError, ExitStatus};
use output::{EnvelopeMeta, OutputWriter};

pub fn main_entry() -> ExitCode {
    let cli = Cli::parse();
    let status = match run(cli) {
        Ok(status) => status,
        Err((status, _)) => status,
    };

    ExitCode::from(status.code())
}

fn run(cli: Cli) -> Result<ExitStatus, (ExitStatus, AppError)> {
    let start = Instant::now();
    let command_name = cli.command.name();
    let store = match store::resolve_store_path(cli.store.as_deref()) {
        Ok(store) => store,
        Err(err) => {
            let output = OutputWriter::new(cli.format, cli.quiet);
            let fallback_store = cli
                .store
                .as_deref()
                .map(std::path::PathBuf::from)
                .unwrap_or_else(|| std::path::PathBuf::from("<unresolved>"));
            let status = err.exit_status();
            let _ = output.write_error(&err, command_name, &fallback_store, start.elapsed());
            return Err((status, err));
        }
    };
    let output = OutputWriter::new(cli.format, cli.quiet);

    let result = match command::execute(&cli.command, &store) {
        Ok(result) => result,
        Err(err) => {
            let status = err.exit_status();
            let _ = output.write_error(&err, command_name, &store, start.elapsed());
            return Err((status, err));
        }
    };
    let meta = EnvelopeMeta::new(command_name, start.elapsed(), &store);
    if let Err(err) = output.write_success(&result, meta) {
        let status = err.exit_status();
        return Err((status, err));
    }

    Ok(ExitStatus::Success)
}

#[cfg(test)]
mod tests {
    use std::env;
    use std::fs;
    use std::path::Path;

    use clap::Parser;
    use tempfile::TempDir;

    use crate::cli::{Cli, Command, Format, InitArgs};
    use crate::command;
    use crate::error::ExitStatus;
    use crate::store;

    #[test]
    fn parses_init_store_and_force() {
        let cli = Cli::try_parse_from(["aascribe", "--store", "/tmp/demo", "init", "--force"])
            .expect("cli should parse");

        assert_eq!(cli.store.as_deref(), Some("/tmp/demo"));
        assert!(matches!(
            cli.command,
            Command::Init(InitArgs { force: true })
        ));
    }

    #[test]
    fn parses_json_as_default_format() {
        let cli = Cli::try_parse_from(["aascribe", "list"]).expect("cli should parse");
        assert!(matches!(cli.format, Format::Json));
    }

    #[test]
    fn store_path_prefers_explicit_flag() {
        let temp = TempDir::new().expect("tempdir");
        let explicit = temp.path().join("explicit-store");
        let env_store = temp.path().join("env-store");
        unsafe {
            env::set_var("AASCRIBE_STORE", &env_store);
        }

        let resolved =
            store::resolve_store_path(Some(explicit.to_str().expect("utf8 path"))).expect("path");

        assert_eq!(resolved, explicit);

        unsafe {
            env::remove_var("AASCRIBE_STORE");
        }
    }

    #[test]
    fn store_path_uses_environment_when_present() {
        let temp = TempDir::new().expect("tempdir");
        let env_store = temp.path().join("env-store");
        unsafe {
            env::set_var("AASCRIBE_STORE", &env_store);
        }

        let resolved = store::resolve_store_path(None).expect("path");

        assert_eq!(resolved, env_store);

        unsafe {
            env::remove_var("AASCRIBE_STORE");
        }
    }

    #[test]
    fn init_creates_expected_layout() {
        let temp = TempDir::new().expect("tempdir");
        let store = temp.path().join("aascribe-store");

        let result = command::execute(&Command::Init(InitArgs { force: false }), &store)
            .expect("init should succeed");
        let data = result.data.expect("success data");

        assert_eq!(data["store"], store.to_string_lossy().to_string());
        assert_eq!(data["created"], true);
        assert_eq!(data["reinitialized"], false);
        assert!(store.join("short_term").is_dir());
        assert!(store.join("long_term").is_dir());
        assert!(store.join("index").is_dir());
        assert!(store.join("cache").is_dir());
        assert!(store.join("layout.json").is_file());
    }

    #[test]
    fn init_fails_without_force_when_store_exists() {
        let temp = TempDir::new().expect("tempdir");
        let store = temp.path().join("aascribe-store");
        fs::create_dir_all(&store).expect("create store");

        let err = command::execute(&Command::Init(InitArgs { force: false }), &store)
            .expect_err("init should fail");

        assert_eq!(err.exit_status(), ExitStatus::GeneralRuntimeError);
        assert_eq!(err.code(), "STORE_ALREADY_EXISTS");
    }

    #[test]
    fn init_reinitializes_when_force_is_set() {
        let temp = TempDir::new().expect("tempdir");
        let store = temp.path().join("aascribe-store");
        fs::create_dir_all(&store).expect("create store");
        fs::create_dir_all(store.join("short_term")).expect("create managed dir");
        fs::write(store.join("short_term").join("old.txt"), "stale").expect("write file");
        fs::write(store.join("layout.json"), "{\"layout_version\":\"old\"}").expect("write layout");

        let result = command::execute(&Command::Init(InitArgs { force: true }), &store)
            .expect("init should succeed");
        let data = result.data.expect("success data");

        assert_eq!(data["created"], false);
        assert_eq!(data["reinitialized"], true);
        assert!(!store.join("short_term").join("old.txt").exists());
        assert!(store.join("layout.json").is_file());
    }

    #[test]
    fn init_reports_text_and_json_shape_fields() {
        let temp = TempDir::new().expect("tempdir");
        let store = temp.path().join("aascribe-store");

        let result = command::execute(&Command::Init(InitArgs { force: false }), &store)
            .expect("init should succeed");
        let data = result.data.expect("success data");

        assert_json_field(&data, "layout_version");
        assert_eq!(
            result.text,
            format!("Initialized aascribe store at {}", store.to_string_lossy())
        );
    }

    #[test]
    fn stubbed_command_returns_not_implemented_error() {
        let temp = TempDir::new().expect("tempdir");
        let store = temp.path().join("aascribe-store");

        let err = command::execute(&Command::List(Default::default()), &store)
            .expect_err("command should fail");

        assert_eq!(err.exit_status(), ExitStatus::GeneralRuntimeError);
        assert_eq!(err.code(), "NOT_IMPLEMENTED");
    }

    fn assert_json_field(data: &serde_json::Value, field: &str) {
        assert!(
            data.get(field).is_some(),
            "expected field {field} in {data:?}"
        );
    }

    #[allow(dead_code)]
    fn assert_exists(path: &Path) {
        assert!(path.exists(), "expected {} to exist", path.display());
    }
}
