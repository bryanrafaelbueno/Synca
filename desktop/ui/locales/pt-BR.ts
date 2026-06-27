// Portuguese (Brazil) translations for Synca
import type { TranslationKey } from './en'

const ptBR: Record<TranslationKey, string> = {
  // Navigation
  nav_files: 'Arquivos',
  nav_settings: 'Configurações',
  nav_github: 'GitHub',

  // Settings — sections
  settings_title: 'Configurações',
  settings_account: 'Conta',
  settings_storage: 'Armazenamento',
  settings_language: 'Idioma',
  settings_proxy: 'Proxy',
  settings_ignored_folders: 'Pastas Ignoradas',
  settings_startup: 'Inicialização',
  settings_about: 'Sobre',

  // Account
  account_signed_in_as: 'Conectado como',
  account_used: 'usado',
  account_of: 'de',
  account_free: 'livre',
  account_storage_used_of: '{used} usado de {total}',
  account_storage_free: '{free} livre',
  account_sign_out: 'Sair',
  account_sign_out_confirm: 'Tem certeza que deseja sair? O Synca vai parar de sincronizar.',
  account_unknown: 'Conta desconhecida',
  account_loading: 'Carregando informações da conta…',
  account_google_drive: 'Google Drive',

  // Language
  language_label: 'Idioma de exibição',
  language_en: 'Inglês',
  language_pt_br: 'Português (Brasil)',

  // Proxy
  proxy_mode: 'Modo de proxy',
  proxy_mode_none: 'Sem proxy',
  proxy_mode_system: 'Usar proxy do sistema',
  proxy_mode_manual: 'Manual',
  proxy_type: 'Tipo de proxy',
  proxy_type_socks: 'SOCKS',
  proxy_type_http: 'HTTP',
  proxy_host: 'Servidor proxy',
  proxy_port: 'Porta',
  proxy_username: 'Usuário (opcional)',
  proxy_password: 'Senha (opcional)',
  proxy_ignore_certificates: 'Ignorar erros de certificado TLS',
  proxy_ignore_certificates_desc: 'Use apenas com um proxy SOCKS confiável que intercepta certificados HTTPS.',
  proxy_save: 'Salvar configurações de proxy',
  proxy_saved: 'Salvo!',
  proxy_error_title: 'Erro de proxy:',
  proxy_error_status: 'Erro de proxy',

  // Ignored Folders
  ignored_folders_desc: 'Arquivos dentro dessas pastas serão excluídos da sincronização.',
  ignored_folders_placeholder: 'ex: node_modules, .git, dist',
  ignored_folders_add: 'Adicionar',
  ignored_folders_empty: 'Nenhuma pasta ignorada ainda.',

  // Startup
  startup_on_boot: 'Iniciar com o sistema',
  startup_on_boot_desc: 'Inicia o Synca automaticamente ao fazer login.',

  // About
  about_version: 'Versão',
  about_github: 'Ver no GitHub',
  about_license: 'Licença',

  // Danger zone
  danger_zone: 'Zona de Perigo',
  danger_sign_out_title: 'Sair do Google Drive',
  danger_sign_out_desc: 'Remove suas credenciais salvas. O Synca vai parar e pedir para fazer login novamente.',

  // Sidebar
  sidebar_storage: 'Armazenamento',
  sidebar_synced: 'sincronizado',
  sidebar_progress: 'Progresso',
  sidebar_files_count: 'arquivos',
  sidebar_drive_limit: 'Limite do Drive: máximo de 100 pastas aninhadas atingido.',
  sidebar_last_sync: 'Última sincronização',
  sidebar_refresh: 'Recarregar',
  sidebar_refresh_title: 'Reiniciar o daemon',

  // File list
  files_title: 'Arquivos',
  files_search_placeholder: 'Buscar arquivos…',
  files_add_folder: 'Pasta',
  files_add_folder_title: 'Adicionar pasta para sincronizar',
  files_conflict_one: 'conflito',
  files_conflict_many: 'conflitos',
  files_error_one: 'erro',
  files_error_many: 'erros',
  files_connecting_title: 'Conectando ao daemon…',
  files_connecting_sub: 'Aguarde',
  files_empty_title: 'Nenhum arquivo encontrado',
  files_empty_search_sub: 'Tente outro termo de busca',
  files_empty_sub: 'Clique em Pasta ao lado da busca ou use synca watch ~/pasta no terminal',
  files_remove_title: 'Remover Pasta da Sincronização',
  files_remove_message: 'Tem certeza que deseja parar de sincronizar esta pasta?\n\nIsso vai REMOVER todos os arquivos do Google Drive, mas manter seus arquivos locais intactos.',
  files_remove_button_title: 'Remover da sincronização (exclui do Drive)',
  files_change_mode_title: 'Clique para alterar o modo de sincronização',
  files_choose_folder_title: 'Escolha a pasta para sincronizar',
  files_folder_picker_unavailable: 'O seletor nativo de pastas não está disponível.\n\nDigite o caminho absoluto da pasta para sincronizar:\n\nLinux:  /home/user/Documentos\nWindows: C:\\Users\\user\\Documents',
  files_choose_sync_mode: 'Escolha o Modo de Sincronização',
  files_cancel: 'Cancelar',
  files_add_folder_confirm: 'Adicionar Pasta',
  files_remove_ignored_title: 'Remover',

  // Sync modes
  sync_mode_two_way: 'Bidirecional',
  sync_mode_two_way_desc: 'Sincroniza nos dois sentidos',
  sync_mode_upload_only: 'Apenas Upload',
  sync_mode_upload_only_desc: 'Local → Drive apenas',
  sync_mode_download_only: 'Apenas Download',
  sync_mode_download_only_desc: 'Drive → Local apenas',

  // Status labels
  status_synced: 'sincronizado',
  status_initializing: 'iniciando…',
  status_uploading: 'enviando…',
  status_verifying: 'verificando…',
  status_finalizing: 'finalizando…',
  status_queued: 'na fila',
  status_conflict: 'conflito',
  status_error: 'erro',

  // Connection
  connect_starting_title: 'Iniciando Synca...',
  connect_daemon_title: 'Conectando ao daemon...',
  connect_initial_setup_title: 'Configuração Inicial',
  connect_auth_required_title: 'Autenticação Necessária',
  connect_pending_title: 'Conexão Pendente',
  connect_saved_settings: 'Procurando configurações salvas...',
  connect_waiting_daemon: 'Aguardando o daemon iniciar...',
  connect_authorize: 'Faça login no Google Drive para autorizar o Synca.',
  connect_starting_daemon: 'Iniciando daemon, aguarde...',
  connect_authenticating: 'Autenticando na Web...',
  connect_login: 'Entrar no Google Drive',
  connect_reload: 'Recarregar',
  connect_login_failed: 'Falha no login: ',
  status_daemon_connected: 'Daemon conectado',
  status_daemon_disconnected: 'Daemon desconectado',
}

export default ptBR
