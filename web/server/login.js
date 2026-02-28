// Cimbar Web Decoder - Login and Session Management

(function () {
    "use strict";

    // API endpoints
    const API_LOGIN = "/api/login";
    const API_LOGOUT = "/api/logout";
    const API_VALIDATE = "/api/session/validate";
    const API_USER = "/api/user";

    // State
    let currentToken = null;
    let currentUserEmail = null;
    let isLoggedIn = false;

    // DOM elements
    const loginModal = document.getElementById("login-modal");
    const loginForm = document.getElementById("login-form");
    const loginEmail = document.getElementById("login-email");
    const loginError = document.getElementById("login-error");
    const userEmailSpan = document.getElementById("user-email");
    const logoutBtn = document.getElementById("logout-btn");

    // Get token from cookie
    function getCookieToken() {
        const cookies = document.cookie.split(';');
        for (let cookie of cookies) {
            const [name, value] = cookie.trim().split('=');
            if (name === 'session_token' && value) {
                return value;
            }
        }
        return null;
    }

    // Check if user is already logged in (via cookie)
    async function checkSession() {
        const token = getCookieToken();
        if (!token) {
            return false;
        }

        try {
            const response = await fetch(API_VALIDATE);
            if (response.ok) {
                const data = await response.json();
                if (data.valid) {
                    currentToken = token;
                    currentUserEmail = data.email;
                    isLoggedIn = true;
                    return true;
                }
            }
        } catch (err) {
            console.error("Session validation failed:", err);
        }

        // Invalid session, clear token
        currentToken = null;
        currentUserEmail = null;
        isLoggedIn = false;
        return false;
    }

    // Login with email
    async function login(email) {
        try {
            const response = await fetch(API_LOGIN, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json"
                },
                body: JSON.stringify({ email })
            });

            const data = await response.json();

            if (!response.ok) {
                throw new Error(data.error || "Login failed");
            }

            // Token is now stored in cookie by server
            currentToken = data.session_token;
            currentUserEmail = email;
            isLoggedIn = true;

            return { success: true, token: data.session_token };
        } catch (err) {
            return { success: false, error: err.message };
        }
    }

    // Logout
    async function logout() {
        if (!currentToken && !getCookieToken()) {
            clearSession();
            return;
        }

        try {
            await fetch(API_LOGOUT, {
                method: "POST"
            });
        } catch (err) {
            console.error("Logout failed:", err);
        }

        clearSession();
    }

    // Clear session state
    function clearSession() {
        currentToken = null;
        currentUserEmail = null;
        isLoggedIn = false;
    }

    // Show login modal
    function showLoginModal() {
        if (loginModal) {
            loginModal.style.display = "flex";
            loginEmail.focus();
        }
    }

    // Hide login modal
    function hideLoginModal() {
        if (loginModal) {
            loginModal.style.display = "none";
        }
    }

    // Update user info display
    function updateUserInfo() {
        if (userEmailSpan) {
            userEmailSpan.textContent = currentUserEmail || "未登录";
        }
    }

    // Get current session token (for capture.js to use)
    function getSessionToken() {
        return getCookieToken() || currentToken;
    }

    // Check if user is logged in
    function getIsLoggedIn() {
        return isLoggedIn;
    }

    // Setup event listeners
    function setupEventListeners() {
        // Login form submit
        if (loginForm) {
            loginForm.addEventListener("submit", async (e) => {
                e.preventDefault();

                const email = loginEmail.value.trim();
                if (!email) {
                    loginError.textContent = "请输入邮箱地址";
                    return;
                }

                loginError.textContent = "";
                const submitBtn = loginForm.querySelector('button[type="submit"]');
                submitBtn.disabled = true;
                submitBtn.textContent = "登录中...";

                const result = await login(email);

                if (result.success) {
                    hideLoginModal();
                    updateUserInfo();
                    // Notify capture.js to connect WebSocket
                    if (window.onLoginSuccess) {
                        window.onLoginSuccess();
                    }
                } else {
                    loginError.textContent = result.error;
                }

                submitBtn.disabled = false;
                submitBtn.textContent = "登录";
            });
        }

        // Logout button
        if (logoutBtn) {
            logoutBtn.addEventListener("click", async () => {
                await logout();
                showLoginModal();
                // Notify capture.js to disconnect WebSocket
                if (window.onLogout) {
                    window.onLogout();
                }
            });
        }

        // Clear all files button
        const clearAllBtn = document.getElementById("clear-all-btn");
        if (clearAllBtn) {
            clearAllBtn.addEventListener("click", async () => {
                if (!confirm("确定要清除所有文件吗？")) {
                    return;
                }

                try {
                    const response = await fetch("/api/files", {
                        method: "DELETE"
                    });

                    if (response.ok) {
                        // Refresh file list
                        if (typeof window.loadCompletedFiles === "function") {
                            await window.loadCompletedFiles();
                        }
                        alert("文件已清除");
                    } else {
                        alert("清除失败");
                    }
                } catch (err) {
                    console.error("Clear files failed:", err);
                    alert("清除失败：" + err.message);
                }
            });
        }
    }

    // Initialize login flow
    async function init() {
        setupEventListeners();

        // Check if already logged in (via cookie)
        const isValid = await checkSession();

        if (isValid) {
            isLoggedIn = true;
            updateUserInfo();
            hideLoginModal();
            // Notify capture.js to connect WebSocket
            if (window.onLoginSuccess) {
                window.onLoginSuccess();
            }
        } else {
            showLoginModal();
        }
    }

    // Export functions for global access
    window.getSessionToken = getSessionToken;
    window.getIsLoggedIn = getIsLoggedIn;
    window.getCurrentUserEmail = () => currentUserEmail;
    window.login = login;
    window.logout = logout;
    window.checkSession = checkSession;

    // Start when DOM is ready
    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", init);
    } else {
        init();
    }
})();
