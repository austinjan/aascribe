use std::io;
use std::path::PathBuf;

use thiserror::Error;

#[allow(dead_code)]
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ExitStatus {
    Success,
    GeneralRuntimeError,
    InvalidArguments,
    StoreNotInitialized,
    NotFound,
}

impl ExitStatus {
    pub fn code(self) -> u8 {
        match self {
            Self::Success => 0,
            Self::GeneralRuntimeError => 1,
            Self::InvalidArguments => 2,
            Self::StoreNotInitialized => 3,
            Self::NotFound => 4,
        }
    }
}

#[allow(dead_code)]
#[derive(Debug, Error)]
pub enum AppError {
    #[error("Could not determine a home directory for the default store path.")]
    HomeDirectoryUnavailable,

    #[error("No store at {path}. Run `aascribe init`.")]
    StoreNotFound { path: PathBuf },

    #[error("A store already exists at {path}. Re-run with `--force` to reinitialize it.")]
    StoreAlreadyExists { path: PathBuf },

    #[error("The command `{command}` is not implemented yet.")]
    NotImplemented { command: &'static str },

    #[error("{message}")]
    Io {
        message: String,
        #[source]
        source: io::Error,
    },

    #[error("Failed to serialize command output.")]
    Serialization(#[from] serde_json::Error),
}

impl AppError {
    pub fn code(&self) -> &'static str {
        match self {
            Self::HomeDirectoryUnavailable => "HOME_DIRECTORY_UNAVAILABLE",
            Self::StoreNotFound { .. } => "STORE_NOT_FOUND",
            Self::StoreAlreadyExists { .. } => "STORE_ALREADY_EXISTS",
            Self::NotImplemented { .. } => "NOT_IMPLEMENTED",
            Self::Io { .. } => "IO_ERROR",
            Self::Serialization(_) => "SERIALIZATION_ERROR",
        }
    }

    pub fn exit_status(&self) -> ExitStatus {
        match self {
            Self::StoreNotFound { .. } => ExitStatus::StoreNotInitialized,
            _ => ExitStatus::GeneralRuntimeError,
        }
    }
}

pub type AppResult<T> = Result<T, AppError>;
