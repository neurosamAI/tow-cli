// Tow Deploy — JetBrains Plugin
// by neurosam.AI — https://neurosam.ai

package ai.neurosam.tow.actions

import com.intellij.openapi.actionSystem.AnAction
import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.ui.Messages

class DeployAction : AnAction() {
    override fun actionPerformed(e: AnActionEvent) {
        val project = e.project ?: return
        val env = Messages.showInputDialog(project, "Environment:", "Tow Deploy", null) ?: return
        val mod = Messages.showInputDialog(project, "Module:", "Tow Deploy", null) ?: return
        TowTerminalRunner.run(project, "tow deploy -e $env -m $mod")
    }
}

class AutoDeployAction : AnAction() {
    override fun actionPerformed(e: AnActionEvent) {
        val project = e.project ?: return
        val env = Messages.showInputDialog(project, "Environment:", "Tow Auto Deploy", null) ?: return
        val mod = Messages.showInputDialog(project, "Module:", "Tow Auto Deploy", null) ?: return
        TowTerminalRunner.run(project, "tow auto -e $env -m $mod")
    }
}

class RollbackAction : AnAction() {
    override fun actionPerformed(e: AnActionEvent) {
        val project = e.project ?: return
        val env = Messages.showInputDialog(project, "Environment:", "Tow Rollback", null) ?: return
        val mod = Messages.showInputDialog(project, "Module:", "Tow Rollback", null) ?: return
        TowTerminalRunner.run(project, "tow rollback -e $env -m $mod")
    }
}

class StatusAction : AnAction() {
    override fun actionPerformed(e: AnActionEvent) {
        val project = e.project ?: return
        val env = Messages.showInputDialog(project, "Environment:", "Tow Status", null) ?: return
        val mod = Messages.showInputDialog(project, "Module:", "Tow Status", null) ?: return
        TowTerminalRunner.run(project, "tow status -e $env -m $mod")
    }
}

class LogsAction : AnAction() {
    override fun actionPerformed(e: AnActionEvent) {
        val project = e.project ?: return
        val env = Messages.showInputDialog(project, "Environment:", "Tow Logs", null) ?: return
        val mod = Messages.showInputDialog(project, "Module:", "Tow Logs", null) ?: return
        TowTerminalRunner.run(project, "tow logs -e $env -m $mod -n 200")
    }
}
