package com.agentbell;

import com.intellij.openapi.Disposable;
import com.intellij.openapi.application.ApplicationManager;
import com.intellij.openapi.diagnostic.Logger;
import com.intellij.openapi.project.Project;
import com.intellij.openapi.project.ProjectManager;
import com.intellij.openapi.wm.ToolWindow;
import com.intellij.openapi.wm.ToolWindowManager;
import com.intellij.openapi.wm.WindowManager;
import com.intellij.ui.content.Content;
import com.intellij.ui.AppIcon;
import com.intellij.terminal.ui.TerminalWidget;
import com.intellij.terminal.frontend.toolwindow.TerminalToolWindowTabsManager;
import com.intellij.terminal.frontend.view.TerminalView;
import org.jetbrains.plugins.terminal.ShellTerminalWidget;
import org.jetbrains.plugins.terminal.TerminalToolWindowManager;
import org.jetbrains.plugins.terminal.session.TerminalStartupOptions;
import kotlinx.coroutines.Deferred;
import javax.swing.JFrame;
import java.awt.Frame;
import java.nio.file.Path;
import java.util.*;
import java.util.concurrent.*;

public final class AgentBellService implements Disposable {
    private static final Logger LOG = Logger.getInstance(AgentBellService.class);
    private Bridge bridge;
    private record Session(String id, String title, Project project, Content content, Runnable focus) {}

    public AgentBellService() {
        try {
            bridge = new Bridge(Path.of(System.getProperty("user.home"), ".cache", "agentbell", "jetbrains"), this::request);
            LOG.info("AgentBell bridge loaded: " + bridge.instanceID);
        } catch (Exception e) {
            LOG.warn("AgentBell bridge unavailable", e);
        }
    }
    public String instanceID() { return bridge == null ? null : bridge.instanceID; }

    private Object request(Bridge.Request request) {
        CompletableFuture<Object> result = new CompletableFuture<>();
        long deadline = System.nanoTime() + 1_500_000_000L;
        ApplicationManager.getApplication().invokeLater(() -> {
            if (System.nanoTime() > deadline || bridge == null) { result.complete(Map.of("ok", false)); return; }
            try {
                List<Session> sessions = sessions();
                if ("list".equals(request.operation())) {
                    List<Object> contexts = new ArrayList<>();
                    for (Session s : sessions) contexts.add(Map.of("ContextID", s.id(), "Title", s.title(), "Selected", s.content().isSelected()));
                    result.complete(Map.of("ok", true, "contexts", contexts));
                } else {
                    for (Session s : sessions) {
                        if (s.id().equals(request.context())) {
                            ToolWindow window = ToolWindowManager.getInstance(s.project()).getToolWindow("Terminal");
                            if (window == null || s.content().getManager() == null) break;
                            s.content().getManager().setSelectedContent(s.content(), true);
                            JFrame frame = WindowManager.getInstance().getFrame(s.project());
                            if (frame != null) {
                                frame.setExtendedState(frame.getExtendedState() & ~Frame.ICONIFIED);
                                frame.toFront();
                                AppIcon.getInstance().requestFocus(frame);
                            }
                            window.activate(s.focus(), true, true);
                            result.complete(Map.of("ok", true));
                            return;
                        }
                    }
                    result.complete(Map.of("ok", false));
                }
            } catch (Throwable e) {
                LOG.warn("AgentBell context request failed", e);
                result.complete(Map.of("ok", false));
            }
        });
        try { return result.get(1800, TimeUnit.MILLISECONDS); }
        catch (Exception e) { return Map.of("ok", false); }
    }

    private String context(Map<String, String> environment) {
        if (environment == null || instanceID() == null) return null;
        String id = environment.get("AGENTBELL_CONTEXT_ID");
        if (id == null || !id.startsWith(instanceID() + ":")) return null;
        try { UUID.fromString(id.substring(instanceID().length() + 1)); return id; }
        catch (IllegalArgumentException e) { return null; }
    }

    private List<Session> sessions() {
        List<Session> sessions = new ArrayList<>();
        for (Project project : ProjectManager.getInstance().getOpenProjects()) {
            if (project.isDisposed()) continue;
            ToolWindow toolWindow = ToolWindowManager.getInstance(project).getToolWindow("Terminal");
            if (toolWindow == null || toolWindow.getContentManagerIfCreated() == null) continue;
            // Classic engine: the per-session startup options retain injected env.
            TerminalToolWindowManager classic = TerminalToolWindowManager.getInstance(project);
            for (TerminalWidget widget : classic.getTerminalWidgets()) {
                ShellTerminalWidget shell = ShellTerminalWidget.asShellJediTermWidget(widget);
                if (shell == null || shell.getStartupOptions() == null || classic.getContainer(widget) == null) continue;
                String id = context(shell.getStartupOptions().getEnvVariables());
                Content content = classic.getContainer(widget).getContent();
                if (id != null && content.isValid()) sessions.add(new Session(id, content.getDisplayName(), project, content, widget::requestFocus));
            }
            // Reworked engine: startup options are available after shell startup.
            for (var tab : TerminalToolWindowTabsManager.getInstance(project).getTabs()) {
                TerminalView view = tab.getView();
                Deferred<? extends TerminalStartupOptions> startup = view.getStartupOptionsDeferred();
                if (!startup.isCompleted() || startup.isCancelled()) continue;
                String id = context(startup.getCompleted().getEnvVariables());
                Content content = tab.getContent();
                if (id != null && content.isValid()) sessions.add(new Session(id, content.getDisplayName(), project, content,
                        () -> view.getPreferredFocusableComponent().requestFocusInWindow()));
            }
        }
        return sessions;
    }

    @Override public void dispose() {
        if (bridge != null) {
            try { bridge.close(); } catch (Exception e) { LOG.warn(e); }
            bridge = null;
        }
    }
}
