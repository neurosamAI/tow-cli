"use strict";
/**
 * Tow Deploy — VS Code Extension
 * by neurosam.AI — https://neurosam.ai
 */
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
Object.defineProperty(exports, "__esModule", { value: true });
exports.activate = activate;
exports.deactivate = deactivate;
const vscode = __importStar(require("vscode"));
const child_process_1 = require("child_process");
const util_1 = require("util");
const execAsync = (0, util_1.promisify)(child_process_1.exec);
// ─── Activation ───
function activate(context) {
    const towConfig = getTowConfigPath();
    // Register tree views
    const envProvider = new EnvironmentTreeProvider(towConfig);
    const modProvider = new ModuleTreeProvider(towConfig);
    const deployProvider = new DeploymentTreeProvider(towConfig);
    context.subscriptions.push(vscode.window.registerTreeDataProvider('tow.environments', envProvider), vscode.window.registerTreeDataProvider('tow.modules', modProvider), vscode.window.registerTreeDataProvider('tow.deployments', deployProvider));
    // Register commands
    const commands = [
        { id: 'tow.deploy', handler: deployModule },
        { id: 'tow.auto', handler: autoDeploy },
        { id: 'tow.rollback', handler: rollback },
        { id: 'tow.status', handler: checkStatus },
        { id: 'tow.logs', handler: streamLogs },
        { id: 'tow.start', handler: startModule },
        { id: 'tow.stop', handler: stopModule },
        { id: 'tow.login', handler: sshLogin },
        { id: 'tow.refresh', handler: async () => {
                envProvider.refresh();
                modProvider.refresh();
                deployProvider.refresh();
            } },
    ];
    for (const cmd of commands) {
        context.subscriptions.push(vscode.commands.registerCommand(cmd.id, cmd.handler));
    }
    // Status bar
    const config = vscode.workspace.getConfiguration('tow');
    if (config.get('showStatusBar')) {
        const statusBar = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 100);
        statusBar.text = '$(rocket) Tow';
        statusBar.tooltip = 'Tow Deploy — Click to deploy';
        statusBar.command = 'tow.auto';
        statusBar.show();
        context.subscriptions.push(statusBar);
    }
}
function deactivate() { }
// ─── Tree Data Providers ───
class TowTreeItem extends vscode.TreeItem {
    constructor(label, collapsibleState, meta) {
        super(label, collapsibleState);
        this.label = label;
        this.collapsibleState = collapsibleState;
        this.meta = meta;
        if (meta) {
            this.description = meta;
        }
    }
}
class EnvironmentTreeProvider {
    constructor(configPath) {
        this.configPath = configPath;
        this._onDidChangeTreeData = new vscode.EventEmitter();
        this.onDidChangeTreeData = this._onDidChangeTreeData.event;
    }
    refresh() { this._onDidChangeTreeData.fire(undefined); }
    getTreeItem(element) { return element; }
    async getChildren() {
        try {
            const { stdout } = await runTow('list envs', this.configPath);
            return stdout.trim().split('\n').filter(Boolean).map(line => {
                const parts = line.trim().split(/\s+/);
                const name = parts[0];
                const servers = parts.find(p => p.startsWith('servers='))?.split('=')[1] || '?';
                const item = new TowTreeItem(name, vscode.TreeItemCollapsibleState.None, `${servers} servers`);
                item.iconPath = new vscode.ThemeIcon('server-environment');
                item.contextValue = 'environment';
                return item;
            });
        }
        catch {
            return [new TowTreeItem('No tow.yaml found', vscode.TreeItemCollapsibleState.None)];
        }
    }
}
class ModuleTreeProvider {
    constructor(configPath) {
        this.configPath = configPath;
        this._onDidChangeTreeData = new vscode.EventEmitter();
        this.onDidChangeTreeData = this._onDidChangeTreeData.event;
    }
    refresh() { this._onDidChangeTreeData.fire(undefined); }
    getTreeItem(element) { return element; }
    async getChildren() {
        try {
            const { stdout } = await runTow('list modules', this.configPath);
            return stdout.trim().split('\n').filter(Boolean).map(line => {
                const parts = line.trim().split(/\s+/);
                const name = parts[0];
                const typePart = parts.find(p => p.startsWith('type='))?.split('=')[1] || '';
                const portPart = parts.find(p => p.startsWith('port='))?.split('=')[1] || '';
                const item = new TowTreeItem(name, vscode.TreeItemCollapsibleState.None, `${typePart} :${portPart}`);
                item.iconPath = new vscode.ThemeIcon(getModuleIcon(typePart));
                item.contextValue = 'module';
                return item;
            });
        }
        catch {
            return [new TowTreeItem('No tow.yaml found', vscode.TreeItemCollapsibleState.None)];
        }
    }
}
class DeploymentTreeProvider {
    constructor(configPath) {
        this.configPath = configPath;
        this._onDidChangeTreeData = new vscode.EventEmitter();
        this.onDidChangeTreeData = this._onDidChangeTreeData.event;
    }
    refresh() { this._onDidChangeTreeData.fire(undefined); }
    getTreeItem(element) { return element; }
    async getChildren() {
        const config = vscode.workspace.getConfiguration('tow');
        const defaultEnv = config.get('defaultEnvironment');
        if (!defaultEnv) {
            return [new TowTreeItem('Set tow.defaultEnvironment to see history', vscode.TreeItemCollapsibleState.None)];
        }
        try {
            // Get first module for deployment history
            const { stdout: modOut } = await runTow('list modules', this.configPath);
            const firstModule = modOut.trim().split('\n')[0]?.trim().split(/\s+/)[0];
            if (!firstModule) {
                return [];
            }
            const { stdout } = await runTow(`list deployments -e ${defaultEnv} -m ${firstModule} -o json`, this.configPath);
            const deployments = JSON.parse(stdout);
            return deployments.map(d => {
                const label = d.timestamp;
                const item = new TowTreeItem(label, vscode.TreeItemCollapsibleState.None, d.current ? 'current' : '');
                item.iconPath = new vscode.ThemeIcon(d.current ? 'check' : 'history');
                return item;
            });
        }
        catch {
            return [new TowTreeItem('Unable to fetch deployments', vscode.TreeItemCollapsibleState.None)];
        }
    }
}
// ─── Command Handlers ───
async function deployModule() {
    const env = await pickEnvironment();
    if (!env) {
        return;
    }
    const mod = await pickModule();
    if (!mod) {
        return;
    }
    runInTerminal(`tow deploy -e ${env} -m ${mod}`);
}
async function autoDeploy() {
    const env = await pickEnvironment();
    if (!env) {
        return;
    }
    const mod = await pickModule();
    if (!mod) {
        return;
    }
    const strategy = await vscode.window.showQuickPick(['Standard', 'Rolling (one server at a time)', 'Auto-Rollback (revert on failure)'], { placeHolder: 'Deployment strategy' });
    let flags = '';
    if (strategy?.startsWith('Rolling')) {
        flags = '--rolling';
    }
    if (strategy?.startsWith('Auto-Rollback')) {
        flags = '--auto-rollback';
    }
    runInTerminal(`tow auto -e ${env} -m ${mod} ${flags}`.trim());
}
async function rollback() {
    const env = await pickEnvironment();
    if (!env) {
        return;
    }
    const mod = await pickModule();
    if (!mod) {
        return;
    }
    runInTerminal(`tow rollback -e ${env} -m ${mod}`);
}
async function checkStatus() {
    const env = await pickEnvironment();
    if (!env) {
        return;
    }
    const mod = await pickModule();
    if (!mod) {
        return;
    }
    try {
        const configPath = getTowConfigPath();
        const { stdout } = await runTow(`status -e ${env} -m ${mod} -o json`, configPath);
        const statuses = JSON.parse(stdout);
        const panel = vscode.window.createOutputChannel('Tow Status');
        panel.clear();
        panel.appendLine(`Status: ${mod} in ${env}`);
        panel.appendLine('─'.repeat(50));
        for (const s of statuses) {
            const icon = s.status === 'running' ? '●' : '○';
            panel.appendLine(`${icon} server-${s.server} (${s.host})`);
            panel.appendLine(`  Status:     ${s.status}`);
            if (s.pid) {
                panel.appendLine(`  PID:        ${s.pid}`);
            }
            if (s.uptime) {
                panel.appendLine(`  Uptime:     ${s.uptime}`);
            }
            if (s.memory) {
                panel.appendLine(`  Memory:     ${s.memory}`);
            }
            panel.appendLine(`  Deployment: ${s.deployment}`);
            panel.appendLine('');
        }
        panel.show();
    }
    catch (err) {
        vscode.window.showErrorMessage(`Tow status failed: ${err.message}`);
    }
}
async function streamLogs() {
    const env = await pickEnvironment();
    if (!env) {
        return;
    }
    const mod = await pickModule();
    if (!mod) {
        return;
    }
    const filter = await vscode.window.showInputBox({
        prompt: 'Log filter (optional)',
        placeHolder: 'e.g., ERROR, OutOfMemoryError'
    });
    let cmd = `tow logs -e ${env} -m ${mod} -n 200`;
    if (filter) {
        cmd += ` -f "${filter}"`;
    }
    runInTerminal(cmd);
}
async function startModule() {
    const env = await pickEnvironment();
    if (!env) {
        return;
    }
    const mod = await pickModule();
    if (!mod) {
        return;
    }
    runInTerminal(`tow start -e ${env} -m ${mod}`);
}
async function stopModule() {
    const env = await pickEnvironment();
    if (!env) {
        return;
    }
    const mod = await pickModule();
    if (!mod) {
        return;
    }
    runInTerminal(`tow stop -e ${env} -m ${mod}`);
}
async function sshLogin() {
    const env = await pickEnvironment();
    if (!env) {
        return;
    }
    const mod = await pickModule();
    if (!mod) {
        return;
    }
    const server = await vscode.window.showInputBox({
        prompt: 'Server number',
        value: '1'
    });
    runInTerminal(`tow login -e ${env} -m ${mod} -s ${server || '1'}`);
}
// ─── Helpers ───
async function pickEnvironment() {
    try {
        const { stdout } = await runTow('list envs', getTowConfigPath());
        const envs = stdout.trim().split('\n')
            .map(l => l.trim().split(/\s+/)[0])
            .filter(Boolean);
        if (envs.length === 1) {
            return envs[0];
        }
        return vscode.window.showQuickPick(envs, { placeHolder: 'Select environment' });
    }
    catch {
        return vscode.window.showInputBox({ prompt: 'Environment name (e.g., dev, prod)' });
    }
}
async function pickModule() {
    try {
        const { stdout } = await runTow('list modules', getTowConfigPath());
        const modules = stdout.trim().split('\n')
            .map(l => l.trim().split(/\s+/)[0])
            .filter(Boolean);
        if (modules.length === 1) {
            return modules[0];
        }
        return vscode.window.showQuickPick(modules, { placeHolder: 'Select module' });
    }
    catch {
        return vscode.window.showInputBox({ prompt: 'Module name (e.g., api-server)' });
    }
}
function getTowConfigPath() {
    return vscode.workspace.getConfiguration('tow').get('configPath', 'tow.yaml');
}
function runInTerminal(cmd) {
    let terminal = vscode.window.terminals.find(t => t.name === 'Tow');
    if (!terminal) {
        terminal = vscode.window.createTerminal('Tow');
    }
    terminal.show();
    terminal.sendText(cmd);
}
async function runTow(args, configPath) {
    const cwd = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
    return execAsync(`tow ${args} -c ${configPath}`, { cwd });
}
function getModuleIcon(type) {
    const icons = {
        springboot: 'coffee',
        java: 'coffee',
        node: 'symbol-event',
        python: 'symbol-method',
        go: 'symbol-namespace',
        rust: 'gear',
        kafka: 'mail',
        redis: 'database',
        generic: 'package',
    };
    return icons[type] || 'package';
}
//# sourceMappingURL=extension.js.map