import XCTest
@testable import EnergyHub

final class SitePresentationTests: XCTestCase {
    func testFormatReadingWithValueAndUnit() {
        XCTAssertEqual(formatReading(1.24, unit: "kW"), "1,24 kW")
    }

    func testFormatReadingWithValuePercent() {
        XCTAssertEqual(formatReading(72, unit: "%"), "72 %")
    }

    func testFormatReadingNilValueShowsDash() {
        XCTAssertEqual(formatReading(nil, unit: "kW"), "—")
    }

    func testFormatReadingNilValueNoUnitShowsDash() {
        XCTAssertEqual(formatReading(nil, unit: ""), "—")
    }

    func testDeviceStatusLabelWithStatus() {
        let device = DeviceSummary(id: "dev-inversor-1", name: "Inversor solar", kind: "inversor", status: "En línea", note: "Valor de ejemplo")
        XCTAssertEqual(deviceStatusLabel(device), "En línea")
    }

    func testDeviceStatusLabelNilStatusShowsNoData() {
        let device = DeviceSummary(id: "dev-bateria-1", name: "Batería doméstica", kind: "batería", status: nil, note: "Valor de ejemplo")
        XCTAssertEqual(deviceStatusLabel(device), "Sin datos disponibles")
    }

    func testDeviceStatusLabelBlankStatusShowsNoData() {
        let device = DeviceSummary(id: "dev-x", name: "Equipo", kind: "otro", status: "   ", note: "Valor de ejemplo")
        XCTAssertEqual(deviceStatusLabel(device), "Sin datos disponibles")
    }

    func testHasDevicesEmptyIsFalse() {
        XCTAssertFalse(hasDevices([]))
    }

    func testHasDevicesNonEmptyIsTrue() {
        XCTAssertTrue(hasDevices(exampleSite.devices))
    }

    func testExampleSiteHasExpectedContent() {
        XCTAssertEqual(exampleSite.name, "Casa de ejemplo")
        XCTAssertEqual(exampleSite.readings.count, 4)
        XCTAssertEqual(exampleSite.devices.count, 2)
    }
}
