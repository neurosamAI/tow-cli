// Tow Deploy — JetBrains Plugin
// by neurosam.AI — https://neurosam.ai

package ai.neurosam.tow

import com.intellij.openapi.project.Project
import com.intellij.openapi.wm.ToolWindow
import com.intellij.openapi.wm.ToolWindowFactory
import com.intellij.ui.content.ContentFactory
import javax.swing.*
import java.awt.BorderLayout

class TowToolWindowFactory : ToolWindowFactory {

    override fun createToolWindowContent(project: Project, toolWindow: ToolWindow) {
        val panel = TowPanel(project)
        val content = ContentFactory.getInstance().createContent(panel, "Tow", false)
        toolWindow.contentManager.addContent(content)
    }
}

class TowPanel(private val project: Project) : JPanel(BorderLayout()) {

    private val envList = DefaultListModel<String>()
    private val moduleList = DefaultListModel<String>()

    init {
        val tabbedPane = JTabbedPane()

        // Environments tab
        val envPanel = JPanel(BorderLayout())
        val envJList = JList(envList)
        envPanel.add(JScrollPane(envJList), BorderLayout.CENTER)
        envPanel.add(createRefreshButton("Environments") { refreshEnvironments() }, BorderLayout.SOUTH)
        tabbedPane.addTab("Environments", envPanel)

        // Modules tab
        val modPanel = JPanel(BorderLayout())
        val modJList = JList(moduleList)
        modPanel.add(JScrollPane(modJList), BorderLayout.CENTER)
        modPanel.add(createRefreshButton("Modules") { refreshModules() }, BorderLayout.SOUTH)
        tabbedPane.addTab("Modules", modPanel)

        // Actions tab
        val actionsPanel = JPanel()
        actionsPanel.layout = BoxLayout(actionsPanel, BoxLayout.Y_AXIS)
        actionsPanel.add(createActionButton("Auto Deploy") { runTowInTerminal("auto") })
        actionsPanel.add(createActionButton("Deploy") { runTowInTerminal("deploy") })
        actionsPanel.add(createActionButton("Rollback") { runTowInTerminal("rollback") })
        actionsPanel.add(createActionButton("Status") { runTowInTerminal("status") })
        actionsPanel.add(createActionButton("Logs") { runTowInTerminal("logs") })
        actionsPanel.add(createActionButton("Start") { runTowInTerminal("start") })
        actionsPanel.add(createActionButton("Stop") { runTowInTerminal("stop") })
        actionsPanel.add(createActionButton("SSH Login") { runTowInTerminal("login") })
        tabbedPane.addTab("Actions", JScrollPane(actionsPanel))

        add(tabbedPane, BorderLayout.CENTER)
        refreshEnvironments()
        refreshModules()
    }

    private fun refreshEnvironments() {
        envList.clear()
        try {
            val output = runTow("list envs")
            output.lines().filter { it.isNotBlank() }.forEach { envList.addElement(it.trim()) }
        } catch (e: Exception) {
            envList.addElement("Error: ${e.message}")
        }
    }

    private fun refreshModules() {
        moduleList.clear()
        try {
            val output = runTow("list modules")
            output.lines().filter { it.isNotBlank() }.forEach { moduleList.addElement(it.trim()) }
        } catch (e: Exception) {
            moduleList.addElement("Error: ${e.message}")
        }
    }

    private fun runTow(args: String): String {
        val basePath = project.basePath ?: "."
        val process = ProcessBuilder("tow", *args.split(" ").toTypedArray(), "-c", "$basePath/tow.yaml")
            .directory(java.io.File(basePath))
            .redirectErrorStream(true)
            .start()
        return process.inputStream.bufferedReader().readText().also { process.waitFor() }
    }

    private fun runTowInTerminal(command: String) {
        val terminal = com.intellij.terminal.JBTerminalWidget.findWidgetByContent(
            project
        )
        // Fallback: use ProcessBuilder to open in OS terminal
        val basePath = project.basePath ?: "."
        val env = JOptionPane.showInputDialog(this, "Environment:", "dev") ?: return
        val mod = JOptionPane.showInputDialog(this, "Module:") ?: return
        val cmd = "tow $command -e $env -m $mod"

        // Open in IDE terminal
        val terminalView = com.intellij.openapi.wm.ToolWindowManager.getInstance(project)
            .getToolWindow("Terminal")
        terminalView?.show {
            val widget = terminalView.contentManager.selectedContent?.component
            // Send command to terminal
        }
    }

    private fun createRefreshButton(label: String, action: () -> Unit): JButton {
        return JButton("Refresh $label").apply {
            addActionListener { action() }
        }
    }

    private fun createActionButton(label: String, action: () -> Unit): JButton {
        return JButton(label).apply {
            addActionListener { action() }
            maximumSize = java.awt.Dimension(Int.MAX_VALUE, 36)
            alignmentX = LEFT_ALIGNMENT
        }
    }
}
