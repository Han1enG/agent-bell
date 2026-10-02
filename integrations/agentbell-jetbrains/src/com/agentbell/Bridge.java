package com.agentbell;

import com.google.gson.Gson;
import java.io.IOException;
import java.net.StandardProtocolFamily;
import java.net.UnixDomainSocketAddress;
import java.nio.ByteBuffer;
import java.nio.channels.ServerSocketChannel;
import java.nio.channels.SocketChannel;
import java.nio.charset.StandardCharsets;
import java.nio.file.*;
import java.nio.file.attribute.PosixFilePermissions;
import java.util.Map;
import java.util.UUID;
import java.util.function.Function;

/** Private, bounded local IPC. This protocol has no terminal input operation. */
public final class Bridge implements AutoCloseable {
    public record Request(String operation, String context) {}
    private static final Gson JSON = new Gson();
    private final ServerSocketChannel server;
    private final Path socket;
    private final Function<Request, Object> handler;
    public final String instanceID = UUID.randomUUID().toString();
    private volatile boolean closed;

    public Bridge(Path directory, Function<Request, Object> handler) throws IOException {
        this.handler = handler;
        Path home = Path.of(System.getProperty("user.home")).toAbsolutePath();
        directory = directory.toAbsolutePath();
        if (!directory.startsWith(home)) throw new IOException("Bridge directory must be inside home");
        for (Path part = home; ; ) {
            if (Files.exists(part, LinkOption.NOFOLLOW_LINKS)) {
                if (Files.isSymbolicLink(part) || !Files.isDirectory(part)) throw new IOException("Unsafe bridge directory");
            } else {
                Files.createDirectory(part, PosixFilePermissions.asFileAttribute(PosixFilePermissions.fromString("rwx------")));
            }
            if (part.equals(directory)) break;
            part = part.resolve(directory.subpath(part.getNameCount(), part.getNameCount() + 1));
        }
        if (!Files.getOwner(directory).equals(Files.getOwner(home))) throw new IOException("Foreign bridge owner");
        Files.setPosixFilePermissions(directory, PosixFilePermissions.fromString("rwx------"));
        socket = directory.resolve(instanceID + ".sock");
        server = ServerSocketChannel.open(StandardProtocolFamily.UNIX);
        try {
            server.bind(UnixDomainSocketAddress.of(socket));
            Files.setPosixFilePermissions(socket, PosixFilePermissions.fromString("rw-------"));
        } catch (IOException e) {
            server.close();
            Files.deleteIfExists(socket);
            throw e;
        }
        Thread worker = new Thread(this::serve, "AgentBell context bridge");
        worker.setDaemon(true);
        worker.start();
    }

    private void serve() {
        while (!closed) {
            try (SocketChannel client = server.accept()) {
                client.configureBlocking(false);
                ByteBuffer input = ByteBuffer.allocate(8192);
                long deadline = System.nanoTime() + 2_000_000_000L;
                boolean complete = false;
                while (input.hasRemaining() && System.nanoTime() < deadline) {
                    int oldPosition = input.position();
                    int read = client.read(input);
                    if (read < 0) break;
                    for (int i = oldPosition; i < input.position(); i++) {
                        if (input.get(i) == '\n') { input.limit(i); complete = true; break; }
                    }
                    if (complete) break;
                    Thread.sleep(5);
                }
                Object result = Map.of("ok", false);
                if (complete) {
                    input.flip();
                    Request request = JSON.fromJson(StandardCharsets.UTF_8.decode(input).toString(), Request.class);
                    if (request != null && ("list".equals(request.operation()) || "focus".equals(request.operation()))) {
                        result = handler.apply(request);
                    }
                }
                ByteBuffer response = StandardCharsets.UTF_8.encode(JSON.toJson(result) + "\n");
                if (response.remaining() > 65536) response = StandardCharsets.UTF_8.encode("{\"ok\":false}\n");
                deadline = System.nanoTime() + 1_000_000_000L;
                while (response.hasRemaining() && System.nanoTime() < deadline) {
                    client.write(response);
                    if (response.hasRemaining()) Thread.sleep(5);
                }
            } catch (Exception e) {
                if (closed) break;
                // Invalid/disconnected clients cannot terminate the bridge.
            }
        }
    }

    @Override public void close() throws IOException {
        closed = true;
        server.close();
        Files.deleteIfExists(socket);
    }
}
