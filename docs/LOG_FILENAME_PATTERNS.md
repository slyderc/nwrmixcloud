# Log Filename Pattern Guidelines

This document provides comprehensive guidance for creating safe, cross-platform log filename patterns in the Mixcloud Updater application.

## 🛡️ **Safe Patterns (Recommended)**

These patterns work reliably on **all platforms** (Windows, macOS, Linux):

### **Compact Formats**
```toml
filename_pattern = "mixcloud-updater-YYYYMMDD.log"          # 20250630
filename_pattern = "app-YYYYMMDD-HHMMSS.log"               # 20250630-143022
```

### **Readable Formats** 
```toml
filename_pattern = "app-YYYY-MM-DD.log"                    # 2025-06-30
filename_pattern = "app-YYYY-MM-DD-HH-MM.log"              # 2025-06-30-14-30
filename_pattern = "app-YYYY.MM.DD.log"                    # 2025.06.30
filename_pattern = "app_YYYY_MM_DD.log"                    # 2025_06_30
```

### **With Subdirectories**
```toml
filename_pattern = "logs/app-YYYYMMDD.log"                 # logs/20250630.log
filename_pattern = "archive/YYYY/MM/app-DD.log"            # archive/2025/06/app-30.log
```

## ⚠️ **Unsafe Patterns (Avoid)**

### **❌ Path Separators in Date**
```toml
# NEVER USE - Creates invalid filenames
filename_pattern = "app-MM/DD/YYYY.log"      # Creates: app-06/30/2025.log ❌
filename_pattern = "app-DD\\MM\\YYYY.log"    # Creates: app-30\\06\\2025.log ❌
```
**Problem:** Forward slashes and backslashes are path separators, creating subdirectories instead of single filenames.

**✅ Safe Alternative:**
```toml
filename_pattern = "app-MM-DD-YYYY.log"      # Creates: app-06-30-2025.log ✅
```

### **❌ Windows Invalid Characters**
```toml
# Windows will reject these patterns
filename_pattern = "app-HH:MM:SS.log"        # Colon ❌
filename_pattern = "app-YYYY|MM.log"         # Pipe ❌  
filename_pattern = "app-*-YYYYMMDD.log"      # Asterisk ❌
filename_pattern = "app-?-YYYYMMDD.log"      # Question mark ❌
filename_pattern = "app-<YYYY>.log"          # Angle brackets ❌
filename_pattern = "app-\"YYYY\".log"        # Quotes ❌
```

**✅ Safe Alternatives:**
```toml
filename_pattern = "app-HH-MM-SS.log"        # Use dashes ✅
filename_pattern = "app-YYYY-MM.log"         # Use dashes ✅
filename_pattern = "app-X-YYYYMMDD.log"      # Use letters ✅
filename_pattern = "app-YYYY.log"            # Remove brackets ✅
```

## 🔧 **Pattern Validation**

The application automatically validates filename patterns and provides helpful error messages:

```bash
# Invalid pattern
filename_pattern = "app-MM/DD/YYYY.log"

# Error message:
invalid filename pattern "app-MM/DD/YYYY.log" contains invalid characters: '/'. 
Suggestion: app-MM-DD-YYYY.log
```

## 🌍 **Cross-Platform Compatibility**

### **Universal Rules (All Platforms)**
- ✅ **Safe characters:** Letters, numbers, hyphens (`-`), underscores (`_`), dots (`.`)
- ❌ **Never use:** Forward slash (`/`), backslash (`\\`), null character (`\\0`)

### **Windows-Specific Rules**
Additional characters that are invalid on Windows:
- ❌ **Invalid:** Colon (`:`), pipe (`|`), asterisk (`*`), question mark (`?`)
- ❌ **Invalid:** Angle brackets (`<`, `>`), quotes (`"`)

### **Unix/Linux/macOS**
- ✅ More permissive than Windows
- ❌ **Still avoid:** Path separators (`/`, `\\`) in filename portions

## 📝 **Date Format Patterns**

### **Supported Date Tokens**
| Token | Description | Example |
|-------|-------------|---------|
| `YYYY` | 4-digit year | `2025` |
| `YY` | 2-digit year | `25` |
| `MM` | 2-digit month (zero-padded) | `06` |
| `M` | 1-2 digit month | `6` |
| `DD` | 2-digit day (zero-padded) | `30` |
| `D` | 1-2 digit day | `30` |

### **Time Tokens (Use with caution)**
| Token | Description | Example | Cross-Platform |
|-------|-------------|---------|----------------|
| `HH` | 2-digit hour | `14` | ✅ Safe |
| `MM` | 2-digit minute | `30` | ✅ Safe |
| `SS` | 2-digit second | `45` | ✅ Safe |

**⚠️ Note:** Avoid `HH:MM:SS` format on Windows. Use `HH-MM-SS` instead.

## 🚀 **Best Practices**

### **1. Use Compact Date Formats**
```toml
# Preferred - compact and unambiguous
filename_pattern = "mixcloud-updater-YYYYMMDD.log"
```

### **2. Include Application Name**
```toml
# Clear identification
filename_pattern = "mixcloud-updater-YYYYMMDD.log"
filename_pattern = "nwr-show-processor-YYYY-MM-DD.log"
```

### **3. Consistent Separators**
```toml
# Pick one separator style and stick with it
filename_pattern = "app-YYYY-MM-DD.log"     # Dashes
filename_pattern = "app_YYYY_MM_DD.log"     # Underscores  
filename_pattern = "app.YYYY.MM.DD.log"     # Dots
```

### **4. Consider Log Rotation**
```toml
# Daily rotation (most common)
filename_pattern = "app-YYYYMMDD.log"

# Hourly rotation (high-volume apps)
filename_pattern = "app-YYYYMMDD-HH.log"

# Monthly rotation (low-volume apps)
filename_pattern = "app-YYYY-MM.log"
```

## 🔍 **Testing Your Patterns**

You can test pattern safety using the built-in validation:

```go
import "github.com/nowwaveradio/mixcloud-updater/internal/logger"

// Test a pattern
err := logger.ValidateFilenamePattern("your-pattern-YYYYMMDD.log")
if err != nil {
    fmt.Printf("Invalid pattern: %v\\n", err)
}

// Get safe pattern examples
safePatterns := logger.GetSafeFilenamePatterns()
fmt.Printf("Safe patterns: %v\\n", safePatterns)

// Get unsafe pattern examples  
unsafePatterns := logger.GetUnsafeFilenamePatterns()
for pattern, reason := range unsafePatterns {
    fmt.Printf("Avoid %s: %s\\n", pattern, reason)
}
```

## 📋 **Migration Guide**

### **From Old Patterns**
If you're migrating from strftime-style patterns:

```toml
# Old format (no longer supported)
filename_pattern = "mixcloud-updater-%Y%m%d.log"

# New format (unified)
filename_pattern = "mixcloud-updater-YYYYMMDD.log"
```

### **Quick Migration Reference**
| Old Pattern | New Pattern | Description |
|-------------|-------------|-------------|
| `%Y%m%d` | `YYYYMMDD` | Compact date |
| `%Y-%m-%d` | `YYYY-MM-DD` | ISO date |
| `%Y.%m.%d` | `YYYY.MM.DD` | Dot separated |
| `%m/%d/%Y` | `MM-DD-YYYY` | US format (safe) |

## 🆘 **Common Issues & Solutions**

### **Issue: "File not found" errors**
**Cause:** Pattern contains path separators  
**Solution:** Replace `/` or `\\` with `-` or `_`

### **Issue: "Access denied" on Windows**
**Cause:** Pattern contains Windows-invalid characters  
**Solution:** Remove or replace `:`, `|`, `*`, `?`, `<`, `>`, `"`

### **Issue: Logs not rotating properly**
**Cause:** Date pattern doesn't change daily  
**Solution:** Include date tokens that change (YYYY, MM, DD)

### **Issue: Too many log files**
**Cause:** Pattern creates new file too frequently  
**Solution:** Use daily (`YYYYMMDD`) instead of hourly patterns

---

## 📚 **Quick Reference**

### **✅ SAFE Patterns**
```toml
filename_pattern = "app-YYYYMMDD.log"           # ⭐ Recommended
filename_pattern = "app-YYYY-MM-DD.log"         # ⭐ Recommended  
filename_pattern = "app-YYYY.MM.DD.log"         # ✅ Good
filename_pattern = "app_YYYY_MM_DD.log"         # ✅ Good
filename_pattern = "logs/app-YYYYMMDD.log"      # ✅ Good
```

### **❌ UNSAFE Patterns**
```toml
filename_pattern = "app-MM/DD/YYYY.log"         # ❌ Path separators
filename_pattern = "app-HH:MM:SS.log"           # ❌ Windows invalid
filename_pattern = "app-YYYY|MM.log"            # ❌ Windows invalid
filename_pattern = "app-*-YYYY.log"             # ❌ Windows invalid
```

Remember: **When in doubt, use `app-YYYYMMDD.log` - it's safe, clear, and works everywhere!** 🎯