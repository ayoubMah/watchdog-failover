package com.upec.victimapp;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class VictimController {

    // We will set this in Docker (e.g., "INSTANCE-A" or "INSTANCE-B")
    @Value("${INSTANCE_ID:UNKNOWN}")
    private String instanceId;

    @GetMapping("/status")
    public String status() {
        // Simple log to console to show we received a request
        System.out.println("PING received on " + instanceId);
        return "I am alive aka أنا على قيد الحياة! ID: " + instanceId;
    }
}
