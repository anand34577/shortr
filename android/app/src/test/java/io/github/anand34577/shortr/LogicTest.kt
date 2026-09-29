package io.github.anand34577.shortr

import io.github.anand34577.shortr.data.ShortrApi
import io.github.anand34577.shortr.ui.common.extractUrl
import io.github.anand34577.shortr.ui.onboarding.isPrivateHost
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class LogicTest {
    @Test fun normalizesWhatPeopleType() {
        assertEquals("https://sho.rt", ShortrApi.normalizeServer("sho.rt"))
        assertEquals("https://sho.rt", ShortrApi.normalizeServer(" https://sho.rt/app/links?x=1 "))
        assertEquals("http://10.0.0.5:8080", ShortrApi.normalizeServer("http://10.0.0.5:8080/"))
        assertNull(ShortrApi.normalizeServer("   "))
    }

    @Test fun recognisesLanAndVpnHosts() {
        assertTrue(isPrivateHost("http://192.168.1.10:8080"))
        assertTrue(isPrivateHost("http://100.64.3.2"))       // Tailscale / Netbird range
        assertTrue(isPrivateHost("http://shortr.home.lan"))
        assertFalse(isPrivateHost("http://sho.rt"))
        assertFalse(isPrivateHost("http://8.8.8.8"))
    }

    @Test fun pullsTheLinkOutOfSharedText() {
        assertEquals("https://example.com/a?b=1", extractUrl("Look at this: https://example.com/a?b=1."))
        assertNull(extractUrl("no link here"))
    }
}
