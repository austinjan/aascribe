use std::io::{self, Write};
use std::path::Path;
use std::time::Duration;

use serde::Serialize;
use serde_json::Value;

use crate::cli::Format;
use crate::error::{AppError, AppResult};

#[derive(Debug, Serialize)]
pub struct EnvelopeMeta {
    command: &'static str,
    duration_ms: u128,
    store: String,
}

impl EnvelopeMeta {
    pub fn new(command: &'static str, elapsed: Duration, store: &Path) -> Self {
        Self {
            command,
            duration_ms: elapsed.as_millis(),
            store: store.to_string_lossy().to_string(),
        }
    }
}

#[derive(Debug)]
pub struct CommandResult {
    pub data: Option<Value>,
    pub text: String,
}

impl CommandResult {
    pub fn new(data: Value, text: impl Into<String>) -> Self {
        Self {
            data: Some(data),
            text: text.into(),
        }
    }
}

pub struct OutputWriter {
    format: Format,
    quiet: bool,
}

impl OutputWriter {
    pub fn new(format: Format, quiet: bool) -> Self {
        Self { format, quiet }
    }

    pub fn write_success(&self, result: &CommandResult, meta: EnvelopeMeta) -> AppResult<()> {
        if self.quiet {
            return Ok(());
        }

        match self.format {
            Format::Json => {
                #[derive(Serialize)]
                struct SuccessEnvelope<'a> {
                    ok: bool,
                    data: &'a Value,
                    meta: EnvelopeMeta,
                }

                let payload = SuccessEnvelope {
                    ok: true,
                    data: result.data.as_ref().expect("success data must exist"),
                    meta,
                };

                print_json(&payload)
            }
            Format::Text => print_line(&result.text),
        }
    }

    pub fn write_error(
        &self,
        err: &AppError,
        command: &'static str,
        store: &Path,
        elapsed: Duration,
    ) -> AppResult<()> {
        if self.quiet {
            return Ok(());
        }

        match self.format {
            Format::Json => {
                #[derive(Serialize)]
                struct ErrorBody<'a> {
                    code: &'a str,
                    message: String,
                }

                #[derive(Serialize)]
                struct ErrorEnvelope<'a> {
                    ok: bool,
                    error: ErrorBody<'a>,
                    meta: EnvelopeMeta,
                }

                let payload = ErrorEnvelope {
                    ok: false,
                    error: ErrorBody {
                        code: err.code(),
                        message: err.to_string(),
                    },
                    meta: EnvelopeMeta::new(command, elapsed, store),
                };

                print_json(&payload)
            }
            Format::Text => print_line(&err.to_string()),
        }
    }
}

fn print_json<T: Serialize>(value: &T) -> AppResult<()> {
    let stdout = io::stdout();
    let mut lock = stdout.lock();
    serde_json::to_writer_pretty(&mut lock, value)?;
    writeln!(lock).map_err(io_to_error("Failed to write JSON output"))?;
    Ok(())
}

fn print_line(line: &str) -> AppResult<()> {
    let stdout = io::stdout();
    let mut lock = stdout.lock();
    writeln!(lock, "{line}").map_err(io_to_error("Failed to write text output"))?;
    Ok(())
}

fn io_to_error(message: &'static str) -> impl FnOnce(io::Error) -> AppError {
    move |source| AppError::Io {
        message: message.to_string(),
        source,
    }
}
