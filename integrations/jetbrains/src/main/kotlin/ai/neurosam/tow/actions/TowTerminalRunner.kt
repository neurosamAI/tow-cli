// Tow Deploy — JetBrains Plugin
// by neurosam.AI — https://neurosam.ai

package ai.neurosam.tow.actions

import com.intellij.openapi.project.Project
import com.intellij.ide.actions.TerminalAction
import org.jetbrains.plugins.terminal.ShellTerminalWidget
import org.jetbrains.plugins.terminal.TerminalToolWindowManager

object TowTerminalRunner {

    fun run(project: Project, command: String) {
        val terminalManager = TerminalToolWindowManager.getInstance(project)
        val terminal = terminalManager.createLocalShellWidget(
            project.basePath ?: ".",
            "Tow"
        )
        terminal.executeCommand(command)
    }
}
