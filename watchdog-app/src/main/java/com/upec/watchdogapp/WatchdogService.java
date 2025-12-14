package com.upec.watchdogapp;

import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;
import org.springframework.web.client.RestTemplate;

@Service
public class WatchdogService {

    private final RestTemplate restTemplate = new RestTemplate();

    // Config: Who to check, and who to wake up
    private final String TARGET_URL = "http://victim-a:9995/status";
    private final String BACKUP_CONTAINER = "victim-b";

    private boolean backupStarted = false;

    // Run this every 5000ms (5 seconds)
    @Scheduled(fixedRate = 5000)
    public void monitor() {
        if (backupStarted) {
            System.out.println(">> System running on Backup. Monitoring paused.");
            return;
        }

        try {
            // 1. Ping Instance A
            restTemplate.getForObject(TARGET_URL, String.class);
            System.out.println(">> Instance A is ALIVE.");

        } catch (Exception e) {
            // 2. If Ping fails (Exception), switch to B
            System.err.println("!! Instance A DOWN. Starting Instance B...");
            startBackup();
        }
    }

    private void startBackup() {
        try {
            // This runs the shell command: "docker start victim-b"
            Process process = new ProcessBuilder("docker", "start", BACKUP_CONTAINER).start();
            process.waitFor(); // Wait for command to finish

            System.out.println("!! Instance B STARTED successfully.");
            backupStarted = true; // Stop checking A

        } catch (Exception e) {
            e.printStackTrace();
        }
    }
}