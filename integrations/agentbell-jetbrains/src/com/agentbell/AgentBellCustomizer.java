package com.agentbell;

import com.intellij.openapi.application.ApplicationManager;
import com.intellij.openapi.project.Project;
import org.jetbrains.plugins.terminal.LocalTerminalCustomizer;
import java.util.Map;
import java.util.UUID;

public final class AgentBellCustomizer extends LocalTerminalCustomizer {
    @Override
    public String[] customizeCommandAndEnvironment(Project project, String workingDirectory,
                                                   String[] command, Map<String, String> environment) {
        AgentBellService service = ApplicationManager.getApplication().getService(AgentBellService.class);
        if (service.instanceID() != null) {
            environment.put("AGENTBELL_SURFACE", "jetbrains");
            environment.put("AGENTBELL_CONTEXT_ID", service.instanceID() + ":" + UUID.randomUUID());
        }
        return command;
    }
}
