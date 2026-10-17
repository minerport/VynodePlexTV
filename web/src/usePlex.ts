import { useCallback, useEffect, useState } from "react";
import {
  loadPlexOverview,
  type PlexServer,
  type PlexLibrary,
  type PlexJob,
} from "./plex-api";
export function usePlex() {
  const [servers, setServers] = useState<PlexServer[]>([]);
  const [libraries, setLibraries] = useState<PlexLibrary[]>([]);
  const [jobs, setJobs] = useState<PlexJob[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const refresh = useCallback(
    () =>
      loadPlexOverview()
        .then((data) => {
          setServers(data.servers);
          setJobs(data.jobs);
          setLibraries(data.libraries);
          setError("");
        })
        .catch((e) => {
          setError(
            e instanceof Error ? e.message : "Could not load Plex status",
          );
        })
        .finally(() => {
          setLoading(false);
        }),
    [],
  );
  useEffect(() => {
    void refresh();
    const interval = window.setInterval(() => {
      if (!document.hidden) void refresh();
    }, 4000);
    return () => window.clearInterval(interval);
  }, [refresh]);
  return { servers, libraries, jobs, error, loading, refresh };
}
