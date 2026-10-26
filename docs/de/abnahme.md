# Eine Website zur Abnahme dokumentieren

Sealjar ist ein freies Open-Source-CLI. Es erfasst ausgewählte Webseiten und
den aktuellen Code eines Git-Repositorys in einem ZIP, versiegelt mit
Prüfsummen (SHA-256) und einem Zeitstempel. So halten Entwickler und Agenturen
zur Abnahme fest, wie Website und Code an diesem Tag aussahen.

Bildschirmfotos zeigen die Desktop- und die Smartphone-Ansicht jeder Seite.
Dazu kommen der gerenderte HTML-Inhalt und ein Bericht in Deutsch und Englisch.
Ein vertrauenswürdiger Zeitstempel zeigt, dass die versiegelten Dateien
spätestens zu diesem Zeitpunkt vorlagen.

Vor der Erfassung muss klar sein, welche Seiten und welcher Code erfasst werden
dürfen und wohin das Paket geschrieben wird. Jede URL wird einzeln angegeben;
Sealjar folgt keinen Links. Ein Paket kann mehrere Hosts und ein Git-Repository
enthalten. Die Erfassung ersetzt weder das Abnahmeprotokoll noch einen
Liefernachweis.

## Installation und Vorschau

Die [Installationsanleitung](../quickstart.md#install) beschreibt Linux, macOS,
Git Bash, WSL und natives Windows. Für Webseiten und die Demo wird Chrome oder
Chromium gebraucht, für Code und die Demo Git. Zum Prüfen eines Pakets, zum
Export der Berichte und zum Wiederherstellen des Codes ist keines von beiden
nötig. Nur zum Auschecken eines History-Bundles braucht es Git. Ohne Chrome
entstehen bei reiner Code-Erfassung HTML-Berichte, aber keine PDFs;
`report --pdf` kopiert nur schon gespeicherte PDFs.

Der Befehl läuft im Stammordner des Repositorys. Das Paket gehört in einen
Ordner außerhalb, zum Beispiel in den übergeordneten Ordner. Die Beispiel-URL
wird durch die freigegebene Adresse ersetzt:

```sh
OUT=$(cd .. && pwd -P)
sealjar capture https://staging.example.com . --out "$OUT" --dry-run
```

Wenn die Vorschau und der Ausgabeordner stimmen, startet die Erfassung mit:

```sh
sealjar capture https://staging.example.com . --out "$OUT" --confirm --no-prompt
```

Die Vorschau zeigt die Seiten, die Code-Dateien, die Ausschlüsse mit Begründung
und den absoluten Ausgabeordner. Relative Pfade beziehen sich auf den aktuellen
Arbeitsordner. Der Trockenlauf startet keinen Browser, greift nicht aufs Netz
zu und schreibt keine Dateien. Bei einer Formularanmeldung ist `--login` nur
mit einer anwesenden Person möglich. HTTP-Basic-Auth wird über `SEALJAR_AUTH`
übergeben; Zugangsdaten gehören weder in URLs noch in Protokolle.
Authorization-Header der Anwendung, etwa Bearer-Tokens für eine API, bleiben
erhalten. Sealjar ergänzt Basic-Auth nur bei Anfragen ohne eigenen
Authorization-Header und nur für die aufgeführten Zielursprünge. Weitere
Optionen stehen in [Capture](../capture.md).

## Ergebnis lesen und prüfen

Neben dem ZIP entsteht ein Leseordner. Der Bericht `report/report.de.html`
verweist relativ auf seine Bilder, deshalb bleibt der Ordner zusammen. Das
Bericht-PDF ist eine eigenständige Lesekopie. Bericht und `SEAL.html` lassen
sich ohne Installation lesen. Für den Zeitstempel verweist der Bericht auf die
Siegelseite und auf `verify`. Die Siegelseite selbst steht nicht in der
Dateiliste des Manifests; beim Export wird sie aus geprüften Daten neu
erstellt.

```sh
sealjar verify paket.sealjar.zip
```

Der Beispielname steht für den tatsächlichen ZIP-Pfad. Die Prüfung gibt drei
getrennte Antworten: Erfassung, Dateien und Zeitstempel. Code 0 bedeutet, dass
die Dateien übereinstimmen und mindestens ein Zeitstempel vertrauenswürdig ist;
Probleme bei der Erfassung können trotzdem vorliegen. Code 10 bedeutet
veränderte oder ungültige Inhalte, Code 11 einen fehlenden vertrauenswürdigen
Zeitstempel. Die Originaldateien bleiben unverändert. Erfasste Dateien werden
nie bearbeitet, damit eine Prüfung besteht.

## Aussagekraft und Datenschutz

Passende Prüfsummen zeigen, dass die Dateien zum Manifest passen. Ein Manifest
ohne Zeitstempel könnte zusammen mit den Dateien ersetzt worden sein. Der
Zeitstempel zeigt, dass die versiegelten Dateien spätestens zu seinem Zeitpunkt
vorlagen; den Zeitpunkt der Erfassung selbst prüft er nicht.

Das Paket zeigt nicht, ob die Inhalte wahr oder vollständig sind. Es zeigt auch
nicht, ob die erfassende Person ehrlich gehandelt hat oder ob die gespeicherten
Daten wirklich vom Server stammen. Es belegt weder Lieferung noch Abnahme, und
es zeigt nicht, ob der erfasste Code die Website erzeugt hat. Eine rechtliche
Wirkung oder die Zulässigkeit als Beweismittel wird nicht zugesagt. Bei
wichtigen Streitfällen empfiehlt sich eine neutrale Begleitung der Erfassung,
etwa durch einen Sachverständigen oder Notar.

Vertrauen entsteht durch die Wurzelzertifikate des Systems, das mitgelieferte
FreeTSA-Root oder separat beschaffte Wurzelzertifikate, die bei der Prüfung
angegeben werden. Zertifikate, die nur im Paket mitkommen, begründen kein
Vertrauen. Eine Offline-Prüfung erkennt spätere Widerrufe nicht. Sealjar v0.1
bewertet keine eIDAS-Qualifikation und bezeichnet keinen Zeitstempel als
qualifiziert. DigiCert ist der Standarddienst, FreeTSA der Ausweichdienst.
Eigene Dienste lassen sich mit `--tsa` angeben. `stamp` ergänzt später einen
Zeitstempel, ohne Manifest oder erfasste Dateien zu ändern.

Bei der Erfassung werden die Zielseiten mit allem geladen, was sie nachladen.
Dazu kommen DNS-Anfragen und die Anfrage an den Zeitstempeldienst, der nur einen
Hash und übliche Netzwerkmetadaten erhält. Zeitstempelanfragen beachten die
üblichen Proxy-Variablen, die Verbindungsproben von `doctor` nicht. Maskierung
und Ausschlüsse entfernen nicht alle privaten Inhalte. Solche Inhalte können in
Bildschirmfotos und HTML stehen, ebenso im Code und in der Git-Historie, die
alte Geheimnisse enthalten kann. Vor der Weitergabe gehört das Paket
durchgesehen.

Bekommt die Demo keinen Online-Zeitstempel, nutzt sie einen lokalen Token mit
dem Hinweis „Demo timestamp, not trusted“. Die Prüfung endet dann mit Code 11.
Der Signaturschlüssel ist öffentlich, deshalb belegt dieser Token keinen
Zeitpunkt, auch wenn ein ausdrücklich angegebenes Wurzelzertifikat ihm
vertraut.

Weiter geht es mit [Ein erhaltenes Paket lesen und prüfen](fuer-empfaenger.md).
