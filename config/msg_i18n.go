package config

// Operator-facing message translation for the startup phase (before the DB connection).
// At this point the settings table cannot be read, so substitution is based on the
// configured default language (Language(): ARTEX_LANG > config file > en) rather than the
// runtime setting (ActiveLanguage). Identifiers needed for remediation (ARTEX_PG_DSN,
// database, dsn, host/user/dbname, etc.) are left untranslated and kept verbatim. "ko" is
// the key itself, so it is not stored; on lookup failure or when the language is ko, the
// original text is returned.
var cfgCatalog = map[string]map[string]string{
	"environment variable ARTEX_PG_DSN": {"ko": "환경 변수 ARTEX_PG_DSN", "zh": "环境变量 ARTEX_PG_DSN", "es": "variable de entorno ARTEX_PG_DSN", "ja": "環境変数 ARTEX_PG_DSN"},
	"config file %s (database.dsn)":     {"ko": "설정 파일 %s (database.dsn)", "zh": "配置文件 %s (database.dsn)", "es": "archivo de configuración %s (database.dsn)", "ja": "設定ファイル %s (database.dsn)"},
	"config file %s (database fields)":  {"ko": "설정 파일 %s (database 필드)", "zh": "配置文件 %s (database 字段)", "es": "archivo de configuración %s (campos database)", "ja": "設定ファイル %s (database フィールド)"},
	"database configuration not found: environment variable ARTEX_PG_DSN is not set, and config file %s has no database (dsn or host/user/dbname) configuration. Create a config file or set the environment variable, then try again": {"ko": "데이터베이스 설정을 찾을 수 없습니다: 환경 변수 ARTEX_PG_DSN 이 설정되어 있지 않고, 설정 파일 %s 에도 database (dsn 또는 host/user/dbname) 설정이 없습니다. 설정 파일을 만들거나 환경 변수를 설정한 뒤 다시 시도하세요", "zh": "找不到数据库配置：环境变量 ARTEX_PG_DSN 未设置，配置文件 %s 中也没有 database（dsn 或 host/user/dbname）配置。请创建配置文件或设置环境变量后重试", "es": "no se encontró la configuración de la base de datos: la variable de entorno ARTEX_PG_DSN no está definida y el archivo de configuración %s tampoco tiene configuración database (dsn o host/user/dbname). Cree un archivo de configuración o defina la variable de entorno e inténtelo de nuevo", "ja": "データベース設定が見つかりません: 環境変数 ARTEX_PG_DSN が設定されておらず、設定ファイル %s にも database (dsn または host/user/dbname) の設定がありません。設定ファイルを作成するか環境変数を設定してから、もう一度お試しください"},
}

// logCatalog holds operator-facing console log templates (startup + runtime
// diagnostics). Keyed by the Korean source template; format verbs are preserved in
// the same positional order so fmt.Printf/log.Printf arguments still line up.
var logCatalog = map[string]map[string]string{
	"[config] config file: %s": {"ko": "[config] 설정 파일: %s", "zh": "[config] 配置文件: %s", "es": "[config] archivo de configuración: %s", "ja": "[config] 設定ファイル: %s"},
	"[config] config file: %s (not found · using only environment variable ARTEX_PG_DSN)": {"ko": "[config] 설정 파일: %s (파일 없음 · 환경 변수 ARTEX_PG_DSN 만 사용)", "zh": "[config] 配置文件: %s (文件不存在 · 仅使用环境变量 ARTEX_PG_DSN)", "es": "[config] archivo de configuración: %s (no encontrado · usando solo la variable de entorno ARTEX_PG_DSN)", "ja": "[config] 設定ファイル: %s (ファイルなし · 環境変数 ARTEX_PG_DSN のみ使用)"},
	"[config] skill directory: %s": {"ko": "[config] skill 디렉터리: %s", "zh": "[config] skill 目录: %s", "es": "[config] directorio de skills: %s", "ja": "[config] スキルディレクトリ: %s"},
	"paused":                       {"ko": "일시정지됨", "zh": "已暂停", "es": "pausada", "ja": "一時停止しました"},
	"resumed":                      {"ko": "재개됨", "zh": "已恢复", "es": "reanudada", "ja": "再開しました"},
	"[auth] moved JWT key from %s to %s (out of the discoverable workspace)":                                                           {"ko": "[auth] JWT 키를 %s 에서 %s 로 이전(탐색 가능한 워크스페이스 밖으로 이동)", "zh": "[auth] 已将 JWT 密钥从 %s 迁移到 %s（移出可被发现的工作区）", "es": "[auth] clave JWT movida de %s a %s (fuera del espacio de trabajo detectable)", "ja": "[auth] JWT キーを %s から %s へ移動しました(探索可能なワークスペースの外へ移動)"},
	"[auth] wrote new JWT key to %s":                                                                                                   {"ko": "[auth] 새 JWT 키를 %s 에 기록", "zh": "[auth] 已将新的 JWT 密钥写入 %s", "es": "[auth] nueva clave JWT escrita en %s", "ja": "[auth] 新しい JWT キーを %s に書き込みました"},
	"[auth] ignoring %s because it is shorter than %d characters; initialise with the setup token instead":                             {"ko": "[auth] %s 가 %d자 미만이라 무시합니다. 설정 토큰 방식으로 초기화하세요", "zh": "[auth] %s 少于 %d 个字符，已忽略。请使用设置令牌方式初始化", "es": "[auth] se ignora %s porque tiene menos de %d caracteres; inicialice con el token de configuración", "ja": "[auth] %s が %d 文字未満のため無視します。設定トークン方式で初期化してください"},
	"[auth] ignoring %s because it exceeds %d bytes; initialise with the setup token instead":                                          {"ko": "[auth] %s 가 %d바이트를 넘어 무시합니다. 설정 토큰 방식으로 초기화하세요", "zh": "[auth] %s 超过 %d 字节，已忽略。请使用设置令牌方式初始化", "es": "[auth] se ignora %s porque supera los %d bytes; inicialice con el token de configuración", "ja": "[auth] %s が %d バイトを超えるため無視します。設定トークン方式で初期化してください"},
	"[auth] could not read the password state; skipping first-run initialisation: %v":                                                  {"ko": "[auth] 비밀번호 상태를 읽지 못해 첫 실행 초기화를 건너뜁니다: %v", "zh": "[auth] 无法读取密码状态，跳过首次运行初始化: %v", "es": "[auth] no se pudo leer el estado de la contraseña; se omite la inicialización inicial: %v", "ja": "[auth] パスワードの状態を読み取れず、初回起動時の初期化をスキップします: %v"},
	"[auth] failed to hash %s: %v":                                                                                                     {"ko": "[auth] %s 해시 생성 실패: %v", "zh": "[auth] 生成 %s 哈希失败: %v", "es": "[auth] no se pudo generar el hash de %s: %v", "ja": "[auth] %s のハッシュ生成に失敗しました: %v"},
	"[auth] failed to store %s: %v":                                                                                                    {"ko": "[auth] %s 저장 실패: %v", "zh": "[auth] 保存 %s 失败: %v", "es": "[auth] no se pudo guardar %s: %v", "ja": "[auth] %s の保存に失敗しました: %v"},
	"[auth] initialised the admin password from environment variable %s (username ARTEX)":                                              {"ko": "[auth] 환경 변수 %s 로 관리자 비밀번호를 초기화했습니다(사용자 이름 ARTEX)", "zh": "[auth] 已通过环境变量 %s 初始化管理员密码（用户名 ARTEX）", "es": "[auth] contraseña de administrador inicializada desde la variable de entorno %s (usuario ARTEX)", "ja": "[auth] 環境変数 %s で管理者パスワードを初期化しました(ユーザー名 ARTEX)"},
	"[auth] the admin password has not been set yet. The setup token is the value of environment variable %s (not printed to the log)": {"ko": "[auth] 관리자 비밀번호가 아직 설정되지 않았습니다. 설정 토큰은 환경 변수 %s 값입니다(로그에 출력하지 않음)", "zh": "[auth] 管理员密码尚未设置。设置令牌为环境变量 %s 的值（不输出到日志）", "es": "[auth] aún no se ha establecido la contraseña de administrador. El token de configuración es el valor de la variable de entorno %s (no se muestra en el registro)", "ja": "[auth] 管理者パスワードがまだ設定されていません。設定トークンは環境変数 %s の値です(ログには出力しません)"},
	"[auth] failed to generate the setup token: %v":                                                                                    {"ko": "[auth] 설정 토큰 생성 실패: %v", "zh": "[auth] 生成设置令牌失败: %v", "es": "[auth] no se pudo generar el token de configuración: %v", "ja": "[auth] 設定トークンの生成に失敗しました: %v"},
	"[auth] the admin password has not been set yet. Setup token: %s":                                                                  {"ko": "[auth] 관리자 비밀번호가 아직 설정되지 않았습니다. 설정 토큰: %s", "zh": "[auth] 管理员密码尚未设置。设置令牌: %s", "es": "[auth] aún no se ha establecido la contraseña de administrador. Token de configuración: %s", "ja": "[auth] 管理者パスワードがまだ設定されていません。設定トークン: %s"},
	"[auth] enter the value above in the \"Setup token\" field on the first screen (/setup), or send it as setup_token in the POST /api/auth/init body. This token is never sent over the network and is only visible in this console log":                                        {"ko": "[auth] 첫 화면(/setup)의 \"설정 토큰\" 칸에 위 값을 입력하거나 POST /api/auth/init 본문의 setup_token 으로 보내십시오. 이 토큰은 네트워크로 전달되지 않으며 이 콘솔 로그에서만 확인할 수 있습니다", "zh": "[auth] 请在首个页面（/setup）的“设置令牌”栏中输入上述值，或将其作为 POST /api/auth/init 请求体中的 setup_token 发送。该令牌不会通过网络传输，只能在此控制台日志中查看", "es": "[auth] introduzca el valor anterior en el campo \"Token de configuración\" de la primera pantalla (/setup) o envíelo como setup_token en el cuerpo de POST /api/auth/init. Este token nunca se transmite por la red y solo puede verse en este registro de consola", "ja": "[auth] 最初の画面(/setup)の\"設定トークン\"欄に上記の値を入力するか、POST /api/auth/init の本文に setup_token として送信してください。このトークンはネットワークでは送信されず、このコンソールログでのみ確認できます"},
	"[auth] warning: the admin password has not been set yet and the server is bound to %q, so other hosts can connect. Initialisation still requires the setup token, but do not expose this port to external networks until initial setup is complete (default 127.0.0.1:8787)": {"ko": "[auth] 경고: 관리자 비밀번호가 아직 설정되지 않았는데 서버가 %q 에 바인딩되어 다른 호스트에서도 접속할 수 있습니다. 설정 토큰 없이는 초기화할 수 없지만, 초기 설정을 마칠 때까지 이 포트를 외부 네트워크에 노출하지 마십시오(기본값 127.0.0.1:8787)", "zh": "[auth] 警告：管理员密码尚未设置，但服务器绑定在 %q，其他主机也可以连接。虽然没有设置令牌无法初始化，但在完成初始设置之前请勿将此端口暴露到外部网络（默认 127.0.0.1:8787）", "es": "[auth] advertencia: aún no se ha establecido la contraseña de administrador y el servidor está enlazado a %q, por lo que otros hosts pueden conectarse. La inicialización sigue requiriendo el token de configuración, pero no exponga este puerto a redes externas hasta completar la configuración inicial (predeterminado 127.0.0.1:8787)", "ja": "[auth] 警告: 管理者パスワードがまだ設定されていないのに、サーバーが %q にバインドされており、他のホストからも接続できます。設定トークンがなければ初期化できませんが、初期設定が完了するまでこのポートを外部ネットワークに公開しないでください(デフォルト 127.0.0.1:8787)"},
	"[auth] initial setup rejected: setup token mismatch (client=%s)":                                              {"ko": "[auth] 초기 설정 거부: 설정 토큰 불일치 (client=%s)", "zh": "[auth] 拒绝初始设置：设置令牌不匹配 (client=%s)", "es": "[auth] configuración inicial rechazada: el token de configuración no coincide (client=%s)", "ja": "[auth] 初期設定を拒否: 設定トークンが一致しません (client=%s)"},
	"[auth] admin password initialised (client=%s)":                                                                {"ko": "[auth] 관리자 비밀번호를 초기화했습니다 (client=%s)", "zh": "[auth] 已初始化管理员密码 (client=%s)", "es": "[auth] contraseña de administrador inicializada (client=%s)", "ja": "[auth] 管理者パスワードを初期化しました (client=%s)"},
	"[auth] password change rejected: current password mismatch (client=%s)":                                       {"ko": "[auth] 비밀번호 변경 거부: 현재 비밀번호 불일치 (client=%s)", "zh": "[auth] 拒绝修改密码：当前密码不匹配 (client=%s)", "es": "[auth] cambio de contraseña rechazado: la contraseña actual no coincide (client=%s)", "ja": "[auth] パスワード変更を拒否: 現在のパスワードが一致しません (client=%s)"},
	"[auth] admin password changed; all existing sessions are invalidated (client=%s)":                             {"ko": "[auth] 관리자 비밀번호를 변경했습니다. 기존 세션은 모두 무효화됩니다 (client=%s)", "zh": "[auth] 已修改管理员密码，所有现有会话均已失效 (client=%s)", "es": "[auth] contraseña de administrador cambiada; todas las sesiones existentes quedan invalidadas (client=%s)", "ja": "[auth] 管理者パスワードを変更しました。既存のセッションはすべて無効化されます (client=%s)"},
	"[auth] login failed (client=%s)":                                                                              {"ko": "[auth] 로그인 실패 (client=%s)", "zh": "[auth] 登录失败 (client=%s)", "es": "[auth] inicio de sesión fallido (client=%s)", "ja": "[auth] ログインに失敗しました (client=%s)"},
	"[notify] background notification loop disabled via %s (test only) — vulnerability IM alerts will not be sent": {"ko": "[notify] 백그라운드 알림 전송 루프가 %s 로 꺼졌습니다(테스트 전용) — 취약점 IM 알림이 전송되지 않습니다", "zh": "[notify] 后台通知发送循环已通过 %s 关闭（仅测试）— 不会发送漏洞 IM 通知", "es": "[notify] el bucle de notificaciones en segundo plano se desactivó mediante %s (solo pruebas) — no se enviarán alertas IM de vulnerabilidades", "ja": "[notify] バックグラウンド通知送信ループが %s により無効化されています(テスト専用) — 脆弱性 IM 通知は送信されません"},
	"[mcp] from %s, discovered and cached %d tool(s)":                                                              {"ko": "[mcp] %s 에서 도구 %d개를 발견해 캐시했습니다", "zh": "[mcp] 从 %s 发现并缓存了 %d 个工具", "es": "[mcp] desde %s, se descubrieron y almacenaron en caché %d herramienta(s)", "ja": "[mcp] %s からツールを %d 件検出し、キャッシュしました"},
	"[mcp] auto-discovery at startup: failed to read the list: %v":                                                 {"ko": "[mcp] 시작 시 자동 발견: 목록을 읽지 못했습니다: %v", "zh": "[mcp] 启动时自动发现：读取列表失败：%v", "es": "[mcp] descubrimiento automático al inicio: no se pudo leer la lista: %v", "ja": "[mcp] 起動時の自動検出: 一覧を読み取れませんでした: %v"},
	"[mcp] auto-discovery of %s at startup failed: %v":                                                             {"ko": "[mcp] 시작 시 %s 자동 발견에 실패했습니다: %v", "zh": "[mcp] 启动时自动发现 %s 失败：%v", "es": "[mcp] falló el descubrimiento automático de %s al inicio: %v", "ja": "[mcp] 起動時の %s の自動検出に失敗しました: %v"},
	"[tools] refreshed orchestration/platform tool schemas to code defaults (one-time)":                            {"ko": "[tools] orchestration/platform 도구 스키마를 코드 기본값으로 새로고침(1회성)", "zh": "[tools] 已将 orchestration/platform 工具 schema 刷新为代码默认值（一次性）", "es": "[tools] esquemas de herramientas orchestration/platform actualizados a los valores predeterminados del código (una sola vez)", "ja": "[tools] orchestration/platform ツールのスキーマをコードのデフォルト値に更新しました(1回限り)"},
	"[reporter] pre-configured the \"%s\" agent + finding trigger":                                                 {"ko": "[reporter] 「%s」 agent + finding 트리거 사전 구성", "zh": "[reporter] 已预配置「%s」agent + finding 触发器", "es": "[reporter] agente \"%s\" + disparador de finding preconfigurados", "ja": "[reporter] 「%s」 agent + finding トリガーを事前構成しました"},
	"[custom-tool] auto-detected python interpreter: %s":                                                           {"ko": "[custom-tool] python 인터프리터 자동 감지: %s", "zh": "[custom-tool] 自动检测到 python 解释器：%s", "es": "[custom-tool] intérprete de python detectado automáticamente: %s", "ja": "[custom-tool] python インタープリターを自動検出: %s"},
	"[pg] database configuration source: %s":                                                                       {"ko": "[pg] 데이터베이스 설정 출처: %s", "zh": "[pg] 数据库配置来源：%s", "es": "[pg] origen de la configuración de la base de datos: %s", "ja": "[pg] データベース設定の取得元: %s"},
	"[mcp] browser MCP capture proxy attached: %s (CA %s)":                                                         {"ko": "[mcp] browser MCP 캡처 프록시 연결: %s (CA %s)", "zh": "[mcp] browser MCP 抓包代理已连接：%s (CA %s)", "es": "[mcp] proxy de captura de browser MCP conectado: %s (CA %s)", "ja": "[mcp] browser MCP キャプチャプロキシを接続: %s (CA %s)"},
	"[mcp] browser MCP capture proxy configuration removed":                                                        {"ko": "[mcp] browser MCP 캡처 프록시 설정 제거", "zh": "[mcp] 已移除 browser MCP 抓包代理配置", "es": "[mcp] configuración del proxy de captura de browser MCP eliminada", "ja": "[mcp] browser MCP キャプチャプロキシの設定を削除しました"},
}

// trCfg translates a startup/operator message to the configured default language
// (Language()). Format templates are translated the same way: pass the Korean
// template through trCfg before fmt.Sprintf/fmt.Errorf.
func trCfg(ko string) string {
	if m, ok := cfgCatalog[ko]; ok {
		if v, ok2 := m[Language()]; ok2 && v != "" {
			return v
		}
	}
	return ko
}

// T translates an operator-facing console log template to the configured default
// language (Language()). Exposed for cmd and server log.Printf call sites. Format
// verbs are preserved in order, so wrap the template only: log.Printf(config.T(ko), args...).
func T(ko string) string {
	if m, ok := logCatalog[ko]; ok {
		if v, ok2 := m[Language()]; ok2 && v != "" {
			return v
		}
	}
	return ko
}
