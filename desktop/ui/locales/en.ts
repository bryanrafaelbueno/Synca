// English (en) translations for Synca
const en = {
  // Navigation
  nav_files: 'Files',
  nav_settings: 'Settings',
  nav_github: 'GitHub',

  // Settings — sections
  settings_title: 'Settings',
  settings_account: 'Account',
  settings_storage: 'Storage',
  settings_language: 'Language',
  settings_proxy: 'Proxy',
  settings_ignored_folders: 'Ignored Folders',
  settings_startup: 'Startup',
  settings_about: 'About',

  // Account
  account_signed_in_as: 'Signed in as',
  account_used: 'used',
  account_of: 'of',
  account_free: 'free',
  account_storage_used_of: '{used} used of {total}',
  account_storage_free: '{free} free',
  account_sign_out: 'Sign out',
  account_sign_out_confirm: 'Are you sure you want to sign out? Synca will stop syncing.',
  account_unknown: 'Unknown account',
  account_loading: 'Loading account info…',
  account_google_drive: 'Google Drive',

  // Language
  language_label: 'Display language',
  language_en: 'English',
  language_pt_br: 'Portuguese (Brazil)',

  // Proxy
  proxy_mode: 'Proxy mode',
  proxy_mode_none: 'No proxy',
  proxy_mode_system: 'Use system proxy',
  proxy_mode_manual: 'Manual',
  proxy_type: 'Proxy type',
  proxy_type_socks: 'SOCKS',
  proxy_type_http: 'HTTP',
  proxy_host: 'Proxy host',
  proxy_port: 'Port',
  proxy_username: 'Username (optional)',
  proxy_password: 'Password (optional)',
  proxy_ignore_certificates: 'Ignore TLS certificate errors',
  proxy_ignore_certificates_desc: 'Use only with a trusted SOCKS proxy that intercepts HTTPS certificates.',
  proxy_save: 'Save proxy settings',
  proxy_saved: 'Saved!',
  proxy_error_title: 'Proxy error:',
  proxy_error_status: 'Proxy error',

  // Ignored Folders
  ignored_folders_desc: 'Files inside these folders will be excluded from syncing.',
  ignored_folders_placeholder: 'e.g. node_modules, .git, dist',
  ignored_folders_add: 'Add',
  ignored_folders_empty: 'No folders ignored yet.',

  // Startup
  startup_on_boot: 'Launch on system startup',
  startup_on_boot_desc: 'Start Synca automatically when you log in.',

  // About
  about_version: 'Version',
  about_github: 'View on GitHub',
  about_license: 'License',

  // Danger zone
  danger_zone: 'Danger Zone',
  danger_sign_out_title: 'Sign out of Google Drive',
  danger_sign_out_desc: 'Removes your saved credentials. Synca will stop and ask you to log in again.',

  // Sidebar
  sidebar_storage: 'Storage',
  sidebar_synced: 'synced',
  sidebar_progress: 'Progress',
  sidebar_files_count: 'files',
  sidebar_drive_limit: 'Drive limit: Max 100 nested folders reached.',
  sidebar_last_sync: 'Last sync',
  sidebar_refresh: 'Refresh',
  sidebar_refresh_title: 'Restart the daemon',

  // File list
  files_title: 'Files',
  files_search_placeholder: 'Search files…',
  files_add_folder: 'Folder',
  files_add_folder_title: 'Add folder to sync',
  files_conflict_one: 'conflict',
  files_conflict_many: 'conflicts',
  files_error_one: 'error',
  files_error_many: 'errors',
  files_connecting_title: 'Connecting to daemon…',
  files_connecting_sub: 'Please wait',
  files_empty_title: 'No files found',
  files_empty_search_sub: 'Try a different search term',
  files_empty_sub: 'Click Folder next to the search bar or use synca watch ~/folder in the terminal',
  files_remove_title: 'Remove Folder from Sync',
  files_remove_message: 'Are you sure you want to stop syncing this folder?\n\nThis will REMOVE all files from Google Drive, but keep your local files untouched.',
  files_remove_button_title: 'Remove from sync (deletes from Drive)',
  files_change_mode_title: 'Click to change sync mode',
  files_choose_folder_title: 'Choose folder to sync',
  files_folder_picker_unavailable: 'Native folder picker is not available.\n\nPlease enter the absolute folder path to sync:\n\nLinux:  /home/user/Documents\nWindows: C:\\Users\\user\\Documents',
  files_choose_sync_mode: 'Choose Sync Mode',
  files_cancel: 'Cancel',
  files_add_folder_confirm: 'Add Folder',
  files_remove_ignored_title: 'Remove',

  // Sync modes
  sync_mode_two_way: 'Two-Way',
  sync_mode_two_way_desc: 'Sync both directions',
  sync_mode_upload_only: 'Upload Only',
  sync_mode_upload_only_desc: 'Local → Drive only',
  sync_mode_download_only: 'Download Only',
  sync_mode_download_only_desc: 'Drive → Local only',

  // Status labels
  status_synced: 'synced',
  status_initializing: 'initializing…',
  status_uploading: 'uploading…',
  status_verifying: 'verifying…',
  status_finalizing: 'finalizing…',
  status_queued: 'queued',
  status_conflict: 'conflict',
  status_error: 'error',

  // Connection
  connect_starting_title: 'Starting Synca...',
  connect_daemon_title: 'Connecting to daemon...',
  connect_initial_setup_title: 'Initial Setup',
  connect_auth_required_title: 'Authentication Required',
  connect_pending_title: 'Connection Pending',
  connect_saved_settings: 'Looking for saved settings...',
  connect_waiting_daemon: 'Waiting for daemon to start...',
  connect_authorize: 'Log in to Google Drive to authorize Synca.',
  connect_starting_daemon: 'Starting daemon, please wait...',
  connect_authenticating: 'Authenticating on the Web...',
  connect_login: 'Log in to Google Drive',
  connect_reload: 'Reload',
  connect_login_failed: 'Login failed: ',
  status_daemon_connected: 'Daemon connected',
  status_daemon_disconnected: 'Daemon disconnected',
} as const

export type TranslationKey = keyof typeof en
export default en
