use std::env;
use std::fs;
use std::path::{Path, PathBuf};

use serde_json::json;

use crate::error::{AppError, AppResult};

const MANAGED_DIRECTORIES: &[&str] = &["short_term", "long_term", "index", "cache"];
const MANAGED_FILES: &[&str] = &["layout.json"];
const LAYOUT_VERSION: &str = "bootstrap-v1";

#[derive(Debug)]
pub struct InitOutcome {
    pub created: bool,
    pub reinitialized: bool,
}

pub fn resolve_store_path(explicit: Option<&str>) -> AppResult<PathBuf> {
    if let Some(path) = explicit {
        return Ok(PathBuf::from(path));
    }

    if let Some(path) = env::var_os("AASCRIBE_STORE") {
        return Ok(PathBuf::from(path));
    }

    let home = dirs::home_dir().ok_or(AppError::HomeDirectoryUnavailable)?;
    Ok(home.join(".aascribe"))
}

pub fn initialize_store(path: &Path, force: bool) -> AppResult<InitOutcome> {
    if path.exists() && !force {
        return Err(AppError::StoreAlreadyExists {
            path: path.to_path_buf(),
        });
    }

    let existed_before = path.exists();
    fs::create_dir_all(path).map_err(io_error_with_path("Failed to create store root", path))?;

    if force {
        reset_managed_contents(path)?;
    }

    for name in MANAGED_DIRECTORIES {
        let dir = path.join(name);
        fs::create_dir_all(&dir)
            .map_err(io_error_with_path("Failed to create store directory", &dir))?;
    }

    let layout_file = path.join("layout.json");
    let layout_doc = json!({
        "layout_version": LAYOUT_VERSION,
        "storage": {
            "short_term": "short_term",
            "long_term": "long_term",
            "index": "index",
            "cache": "cache"
        }
    });
    let layout_bytes = serde_json::to_vec_pretty(&layout_doc)?;
    fs::write(&layout_file, layout_bytes).map_err(io_error_with_path(
        "Failed to write layout metadata",
        &layout_file,
    ))?;

    Ok(InitOutcome {
        created: !existed_before,
        reinitialized: existed_before && force,
    })
}

pub fn layout_version() -> &'static str {
    LAYOUT_VERSION
}

fn reset_managed_contents(root: &Path) -> AppResult<()> {
    for name in MANAGED_DIRECTORIES {
        remove_if_exists(&root.join(name))?;
    }

    for name in MANAGED_FILES {
        remove_if_exists(&root.join(name))?;
    }

    Ok(())
}

fn remove_if_exists(path: &Path) -> AppResult<()> {
    if !path.exists() {
        return Ok(());
    }

    if path.is_dir() {
        fs::remove_dir_all(path).map_err(io_error_with_path(
            "Failed to reset managed directory",
            path,
        ))?;
    } else {
        fs::remove_file(path).map_err(io_error_with_path("Failed to reset managed file", path))?;
    }

    Ok(())
}

fn io_error_with_path(
    message: &'static str,
    path: &Path,
) -> impl FnOnce(std::io::Error) -> AppError {
    let rendered = format!("{message}: {}", path.display());
    move |source| AppError::Io {
        message: rendered,
        source,
    }
}
