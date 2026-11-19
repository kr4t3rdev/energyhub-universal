package com.energyhub.universal

import com.energyhub.universal.site.DeviceSummary
import com.energyhub.universal.site.Reading
import com.energyhub.universal.site.exampleSite
import com.energyhub.universal.site.deviceStatusLabel
import com.energyhub.universal.site.formatReading
import com.energyhub.universal.site.hasDevices
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class SitePresentationTest {
    @Test fun formatReading_withValueAndUnit() {
        assertEquals("1,24 kW", formatReading(1.24, "kW"))
    }

    @Test fun formatReading_withValuePercent() {
        assertEquals("72 %", formatReading(72.0, "%"))
    }

    @Test fun formatReading_nullValueShowsDash() {
        assertEquals("\u2014", formatReading(null, "kW"))
    }

    @Test fun formatReading_nullValueNoUnitShowsDash() {
        assertEquals("\u2014", formatReading(null, ""))
    }

    @Test fun deviceStatusLabel_withStatus() {
        val device = DeviceSummary("dev-inversor-1", "Inversor solar", "inversor", "En l\u00ednea", "Valor de ejemplo")
        assertEquals("En l\u00ednea", deviceStatusLabel(device))
    }

    @Test fun deviceStatusLabel_nullStatusShowsNoData() {
        val device = DeviceSummary("dev-bateria-1", "Bater\u00eda dom\u00e9stica", "bater\u00eda", null, "Valor de ejemplo")
        assertEquals("Sin datos disponibles", deviceStatusLabel(device))
    }

    @Test fun deviceStatusLabel_blankStatusShowsNoData() {
        val device = DeviceSummary("dev-x", "Equipo", "otro", "   ", "Valor de ejemplo")
        assertEquals("Sin datos disponibles", deviceStatusLabel(device))
    }

    @Test fun hasDevices_emptyListIsFalse() {
        assertFalse(hasDevices(emptyList()))
    }

    @Test fun hasDevices_nonEmptyListIsTrue() {
        val device = DeviceSummary("dev-x", "Equipo", "otro", null, "Valor de ejemplo")
        assertTrue(hasDevices(listOf(device)))
    }

    @Test fun exampleSite_matchesWebFixture() {
        assertEquals("Casa de ejemplo", exampleSite.name)
        assertEquals(4, exampleSite.readings.size)
        assertEquals("Potencia solar", exampleSite.readings[0].label)
        assertEquals(1.24, exampleSite.readings[0].value)
        assertEquals("kW", exampleSite.readings[0].unit)
        assertEquals("Estado de red", exampleSite.readings[3].label)
        assertEquals(null, exampleSite.readings[3].value)
        assertEquals(2, exampleSite.devices.size)
        assertEquals("Inversor solar", exampleSite.devices[0].name)
        assertEquals("En l\u00ednea", exampleSite.devices[0].status)
        assertEquals("Bater\u00eda dom\u00e9stica", exampleSite.devices[1].name)
        assertEquals(null, exampleSite.devices[1].status)
    }
}
