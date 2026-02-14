use std::process::Command;
use std::thread;
use std::time::Duration;
use tao::event_loop::{ControlFlow, EventLoop};
use tao::window::WindowBuilder;
use wry::WebViewBuilder;

#[cfg(target_os = "windows")]
use std::os::windows::process::CommandExt;

// ==========================================
//                 常量配置区域
// ==========================================

/// 窗口配置
mod window_config {
    pub const TITLE: &str = "Music DL Desktop";
    pub const WIDTH: f64 = 1280.0;
    pub const HEIGHT: f64 = 800.0;
    // 注意：include_bytes! 宏必须使用字符串字面量，不能使用常量变量
    // 所以这里只定义路径作为注释参考，实际代码中仍需写死
    // pub const ICON_PATH: &str = "../icon.png"; 
}

/// 后端服务配置 (Go程序)
mod server_config {
    pub const PORT: &str = "37777";
    pub const URL_PATH: &str = "/music";
    pub const STARTUP_DELAY_MS: u64 = 2000;
    pub const SHUTDOWN_DELAY_MS: u64 = 500;
    
    // 根据操作系统决定二进制文件名
    #[cfg(target_os = "windows")]
    pub const BINARY_NAME: &str = "music-dl.exe";
    #[cfg(not(target_os = "windows"))]
    pub const BINARY_NAME: &str = "music-dl";

    // 搜索路径列表 (相对路径)
    pub const SEARCH_DIRS: &[&str] = &[
        ".",             // 当前目录
        "..",            // 父目录
        "../..",         // 祖父目录
        "../../.."       // 曾祖父目录
    ];
}

/// 系统/进程相关配置
mod system_config {
    #[cfg(target_os = "windows")]
    pub const CREATE_NO_WINDOW_FLAG: u32 = 0x08000000;
}

// ==========================================
//                 主程序逻辑
// ==========================================

fn main() -> wry::Result<()> {
    // 1. 查找并启动 Go Web 服务
    let mut child = None;
    
    // 动态构建搜索路径
    // 优先搜索预定义的目录，最后尝试从系统 PATH 启动
    let mut possible_paths = Vec::new();
    for dir in server_config::SEARCH_DIRS {
        possible_paths.push(format!("{}/{}", dir, server_config::BINARY_NAME));
    }
    possible_paths.push(server_config::BINARY_NAME.to_string()); // 尝试 PATH

    for path in possible_paths {
        let mut cmd = Command::new(&path);
        cmd.arg("web")
           .arg("--no-browser")
           .arg("-p").arg(server_config::PORT);

        // Windows 专用：隐藏控制台窗口
        #[cfg(target_os = "windows")]
        {
            cmd.creation_flags(system_config::CREATE_NO_WINDOW_FLAG);
        }

        if let Ok(process) = cmd.spawn() {
            println!("Backend server started using: {}", path);
            child = Some(process);
            break;
        }
    }

    // 如果找不到二进制文件，panic
    let mut child = child.expect(&format!(
        "Failed to start backend server. looked for '{}' in search paths.", 
        server_config::BINARY_NAME
    ));

    // 等待服务启动
    thread::sleep(Duration::from_millis(server_config::STARTUP_DELAY_MS));

    // 2. 加载图标
    // include_bytes! 必须使用字面量路径，无法使用 const
    const ICON_DATA: &[u8] = include_bytes!("../icon.png");
    let icon = match image::load_from_memory(ICON_DATA) {
        Ok(img) => {
            let icon_rgba = img.to_rgba8();
            let (width, height) = icon_rgba.dimensions();
            Some(tao::window::Icon::from_rgba(icon_rgba.into_raw(), width, height).unwrap())
        }
        Err(_) => None,
    };

    // 3. 创建窗口
    let event_loop = EventLoop::new();
    let window = WindowBuilder::new()
        .with_title(window_config::TITLE)
        .with_inner_size(tao::dpi::LogicalSize::new(window_config::WIDTH, window_config::HEIGHT))
        .with_window_icon(icon)
        .build(&event_loop)
        .unwrap();

    // 4. 加载 WebView
    // 动态构建 URL：http://localhost:PORT/PATH
    let server_url = format!("http://localhost:{}{}", server_config::PORT, server_config::URL_PATH);
    let _webview = WebViewBuilder::new(&window)
        .with_url(&server_url)
        .build()?;

    // 5. 事件循环
    event_loop.run(move |event, _, control_flow| {
        *control_flow = ControlFlow::Wait;

        match event {
            tao::event::Event::WindowEvent {
                event: tao::event::WindowEvent::CloseRequested,
                ..
            } => {
                println!("Terminating web server...");
                
                // 尝试优雅关闭子进程
                if let Err(e) = child.kill() {
                    eprintln!("Failed to kill child process: {}", e);
                    
                    // Windows 兜底策略：使用 taskkill 强制结束
                    #[cfg(target_os = "windows")]
                    {
                        let _ = std::process::Command::new("taskkill")
                            .args(&["/F", "/IM", server_config::BINARY_NAME])
                            .output();
                    }
                }
                
                thread::sleep(Duration::from_millis(server_config::SHUTDOWN_DELAY_MS));
                println!("Web server terminated. Exiting...");
                *control_flow = ControlFlow::Exit;
            }
            _ => (),
        }
    });
}