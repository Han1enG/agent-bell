import com.agentbell.Bridge;
import java.nio.file.Path;
import java.util.*;

public class BridgeProbe {
    public static void main(String[] args) throws Exception {
        String[] instance = {null};
        String[] selected = {"00000000-0000-4000-8000-000000000001"};
        List<String> tabs = List.of(selected[0], "00000000-0000-4000-8000-000000000002");
        try (Bridge bridge = new Bridge(Path.of(System.getProperty("user.home"), ".cache", "agentbell", "jetbrains"), request -> {
            if ("list".equals(request.operation())) {
                List<Object> contexts = new ArrayList<>();
                for (String tab : tabs) contexts.add(Map.of("ContextID", instance[0] + ":" + tab, "Title", "same title", "Selected", selected[0].equals(tab)));
                return Map.of("ok", true, "contexts", contexts);
            }
            for (String tab : tabs) if ((instance[0] + ":" + tab).equals(request.context())) { selected[0] = tab; return Map.of("ok", true); }
            return Map.of("ok", false);
        })) {
            instance[0] = bridge.instanceID;
            System.out.println(instance[0]);
            System.out.flush();
            System.in.read();
        }
    }
}
