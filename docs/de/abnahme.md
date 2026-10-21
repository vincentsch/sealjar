# Eine Website zur Abnahme dokumentieren

Sealjar hält ausgewählte Webseiten und den aktuellen Code in einem ZIP fest.
Dazu gehören Bildschirmfotos für Desktop und Smartphone, der gerenderte HTML-Inhalt
und ein Erfassungsbericht. Ein unabhängiger Zeitstempel setzt eine zeitliche
Obergrenze dafür, wann die versiegelten Dateien bereits vorlagen.

Vor der Erfassung sollten Umfang, Zugriffsberechtigung und Ausgabeordner feststehen.
Jede URL wird einzeln angegeben; Sealjar folgt keinen Links zur Suche nach
weiteren Seiten. Ein Paket kann mehrere Hosts und ein Git-Repository enthalten.
Die Erfassung ersetzt weder das Abnahmeprotokoll noch einen Liefernachweis.

## Installation und Vorschau

Die [Installationsanleitung](../quickstart.md#install) beschreibt Linux, macOS,
Git Bash, WSL und natives Windows. Für Webseiten und Demo wird Chrome oder
Chromium benötigt, für Code und Demo Git. Prüfung, Berichtsexport und
Snapshot-Wiederherstellung benötigen beides nicht. Ein History-Bundle braucht
Git zum Auschecken. Ohne Chrome sind bei reiner Code-Erfassung HTML-Berichte
möglich, aber keine PDFs. `report --pdf` kopiert nur bereits gespeicherte PDFs.

Im Stammordner des zu erfassenden Repositorys kann die Ausgabe im übergeordneten
Ordner liegen. Der aufgelöste Pfad muss außerhalb des Repositorys liegen.
Die Beispiel-URL wird durch die freigegebene Adresse ersetzt:

```sh
OUT=$(cd .. && pwd -P)
printf 'Ausgabeordner: %s\n' "$OUT"
sealjar capture https://staging.example.com . --out "$OUT" --dry-run
sealjar capture https://staging.example.com . --out "$OUT" --confirm --no-prompt
```

Die vollständige Vorschau zeigt Seiten, Code und Ausschlüsse. Der Ausgabeordner
steht derzeit nicht im CLI-Plan und wird deshalb separat angezeigt. Relative
Pfade beziehen sich auf den aktuellen Arbeitsordner. Der Trockenlauf startet
keinen Browser, greift nicht aufs Netz zu und schreibt keine Dateien.
Bei Formularanmeldung ist `--login` nur mit einer anwesenden Person möglich.
HTTP-Basic-Auth kann über `SEALJAR_AUTH` übergeben werden. Zugangsdaten gehören
weder in URLs noch in Protokolle. Authorization-Header der Anwendung, etwa
Bearer-Tokens für eine API, bleiben erhalten. Sealjar ergänzt Basic-Auth nur
bei Anfragen ohne eigenen Authorization-Header und nur für die aufgeführten
Zielursprünge. Weitere Optionen stehen unter [Capture](../capture.md).

## Ergebnis lesen und prüfen

Neben dem ZIP entsteht ein Leseordner. `report/report.de.html` bleibt zusammen
mit den Bildern in diesem Ordner; die Verweise sind relativ. Das Bericht-PDF
ist eine eigenständige Lesekopie. Bericht und `SEAL.html` sind ohne Installation
lesbar. Der Bericht verweist für Zeitstempel auf die Siegelseite und `verify`.
Die Siegelseite selbst gehört nicht zur Dateiliste im Manifest. Beim Export
wird sie aus geprüften Daten neu erstellt.

```sh
sealjar verify paket.sealjar.zip
```

Der tatsächliche ZIP-Pfad ersetzt den Beispielnamen. Drei Ergebnisse bleiben
getrennt: Erfassung, Dateien und Zeitstempel. Code 0 bedeutet, dass die Dateien
übereinstimmen und mindestens ein Zeitstempel vertrauenswürdig ist. Probleme
bei der Erfassung können trotzdem vorliegen. Code 10 bezeichnet veränderte oder
ungültige Inhalte, Code 11 einen fehlenden vertrauenswürdigen Zeitstempel.
Originaldateien bleiben unverändert; eine fehlgeschlagene Prüfung wird nicht
durch Bearbeiten der erfassten Dateien behoben.

## Aussagekraft und Datenschutz

Passende Prüfsummen (SHA-256) zeigen die Übereinstimmung mit dem Manifest.
Ohne authentifiziertes Manifest könnten Dateien und Prüfsummen gemeinsam ersetzt
worden sein. Ein vertrauenswürdiger Zeitstempel belegt eine zeitliche Obergrenze
für das Vorliegen der versiegelten Dateien, keine unabhängig geprüfte Erfassungszeit.

Das Paket belegt weder Wahrheit noch Vollständigkeit noch ehrliche Bedienung
oder die Herkunft der Bytes vom Server. Auch Lieferung, Abnahme und der Zusammenhang
zwischen erfasstem Code und Website bleiben offen. Eine bestimmte rechtliche
Wirkung oder Zulässigkeit als Beweismittel wird nicht zugesagt. Bei wichtigen
Streitfällen empfiehlt sich eine neutrale Begleitung, etwa durch einen
Sachverständigen oder Notar.

Vertrauen entsteht durch System-Wurzelzertifikate, das eingebettete FreeTSA-Root
oder unabhängig bezogene, ausdrücklich angegebene Wurzeln. Zertifikate allein
aus dem Paket begründen kein Vertrauen. Offline-Prüfungen erkennen spätere
Zertifikatswiderrufe nicht. Sealjar v0.1 bewertet keine eIDAS-Qualifikation
und bezeichnet Zeitstempel nicht als qualifiziert. DigiCert ist der Standard,
FreeTSA der Ausweichdienst. Eigene Dienste lassen sich mit `--tsa` wählen;
`stamp` ergänzt später Zeitstempel, ohne Manifest oder erfasste Dateien zu ändern.

Die Erfassung kontaktiert Zielseiten, deren Ressourcen, DNS und Zeitstempeldienste.
Ein Zeitstempeldienst erhält einen Hash und übliche Netzwerkmetadaten.
Zeitstempelanfragen beachten Standard-Proxys, die Verbindungsproben von `doctor`
nicht. Maskierung und Ausschlüsse entfernen keine privaten Inhalte aus
Bildschirmfotos, DOM, Code oder Historie. Gerade die Historie kann alte Geheimnisse
enthalten. Vor der Weitergabe ist eine Prüfung des Pakets nötig.

Wenn die Demo keinen Online-Zeitstempel erhält, nutzt sie einen lokalen Token
mit dem Hinweis „Demo timestamp, not trusted“. Die Prüfung endet dann mit Code 11.
Der Signaturschlüssel ist öffentlich. Auch wenn ein ausdrücklich angegebenes
Wurzelzertifikat dem Token Vertrauen gewährt, liefert er keinen unabhängigen
Zeitnachweis.

Weiter geht es mit [Pakete lesen und prüfen](fuer-empfaenger.md).
