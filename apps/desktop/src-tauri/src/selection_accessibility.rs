//! Cross-platform text selection access.
//!
//! Windows: UI Automation TextPattern. Reads the *selected* range directly
//! from the focused element, so it never disturbs the clipboard and works
//! across WeChat / browsers / native edit controls. The caller keeps a
//! clipboard-simulation fallback for hosts that expose no TextPattern.
//!
//! The UI Automation COM client (`IUIAutomation`) is expensive to construct
//! (`CoCreateInstance` ~tens of ms), so we build it **once** inside a
//! `SelectionReader` and reuse it for every read on the owning worker thread.
//!
//! macOS/Linux: AXAPI / AT-SPI to follow; `read` returns None for now so the
//! caller falls back to the clipboard.

#[cfg(windows)]
pub struct SelectionReader {
    automation: uiautomation::UIAutomation,
}

#[cfg(windows)]
const MAX_SELECTION_TEXT_CHARS: usize = 8_192;

#[cfg(windows)]
impl SelectionReader {
    /// Build the UI Automation client. Must be called on the thread that will
    /// own it (it initializes COM as MTA on that thread).
    pub fn new() -> Option<Self> {
        match uiautomation::UIAutomation::new() {
            Ok(automation) => Some(Self { automation }),
            Err(e) => {
                log::warn!("UIAutomation init failed: {e}");
                None
            }
        }
    }

    /// Read the currently selected text from the system-focused element.
    pub fn read(&self) -> Option<String> {
        use uiautomation::patterns::UITextPattern;

        let focused = self.automation.get_focused_element().ok()?;
        let pattern = focused.get_pattern::<UITextPattern>().ok()?;
        let ranges = pattern.get_selection().ok()?;

        let mut merged = String::new();
        for range in ranges {
            if let Ok(chunk) = range.get_text(MAX_SELECTION_TEXT_CHARS as i32) {
                let remaining = MAX_SELECTION_TEXT_CHARS.saturating_sub(merged.chars().count());
                if remaining == 0 {
                    break;
                }
                merged.extend(chunk.chars().take(remaining));
            }
        }

        let trimmed = merged.trim().to_string();
        if trimmed.chars().count() >= 2 {
            Some(trimmed)
        } else {
            None
        }
    }
}

#[cfg(not(windows))]
pub struct SelectionReader;

#[cfg(not(windows))]
impl SelectionReader {
    pub fn new() -> Option<Self> {
        Some(SelectionReader)
    }

    pub fn read(&self) -> Option<String> {
        None
    }
}
