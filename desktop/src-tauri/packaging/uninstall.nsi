!macro customUnInstall
  DetailPrint "Stopping Synca processes..."
  # Kill the main UI app
  nsExec::Exec 'taskkill /F /IM Synca.exe /T'
  # Kill the daemon (sidecar)
  nsExec::Exec 'taskkill /F /IM synca-daemon.exe /T'
  # Give it a moment to release file handles
  Sleep 1000
!macroend
