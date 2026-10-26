# Ein erhaltenes Paket lesen und prüfen

Sealjar ist ein freies Open-Source-CLI. Es erfasst ausgewählte Webseiten und
den Code eines Git-Repositorys in einem ZIP, versiegelt mit Prüfsummen
(SHA-256) und einem Zeitstempel. Der Bericht in so einem Paket lässt sich ohne
Installation lesen. Für die Prüfung braucht es Sealjar oder Python 3 und
OpenSSL.

Das ZIP enthält die vollständige Erfassung. Der Leseordner daneben enthält die
Berichte mit ihren Bildern. `report/report.de.html` verweist relativ auf diese
Bilder, deshalb muss der Ordner zusammenbleiben. Das Bericht-PDF ist eine
eigenständige Lesekopie.

Zuerst lohnt sich der Blick auf die Zusammenfassung. Stimmen URLs, Repository
und Commit mit dem erwarteten Umfang überein? Welche Dateien wurden
ausgeschlossen? Jedes festgehaltene Problem zählt, denn eine fehlgeschlagene
Seite oder eine fehlende Ressource hinterlässt eine Lücke. Die Bildschirmfotos
zeigen die Desktop- und die Smartphone-Ansicht beim Laden, samt Cookie-Banner
oder Anmeldeformular. Wie sich die Website bei der Bedienung verhalten hat,
zeigen sie nicht.

Für den Zeitstempel verweist der Bericht auf `SEAL.html` und auf die
Paketprüfung. Die Siegelseite ist eine Lesehilfe. Sie entsteht nach dem
Manifest und steht nicht in dessen Dateiliste; ihr Vertrauensstatus stammt vom
Prüflauf zu diesem Zeitpunkt. `verify` prüft die Token selbst, und
`sealjar report` erstellt die Siegelseite aus geprüften Daten neu.

## Drei getrennte Ergebnisse

Nach der [Installation](../quickstart.md#install) prüft dieser Befehl das
Original-ZIP. Der Beispielname steht für den tatsächlichen Pfad:

```sh
sealjar verify paket.sealjar.zip
```

| Ergebnis | Bedeutung |
| --- | --- |
| Erfassung | Wurde die Erfassung abgeschlossen? Welche Probleme wurden dabei festgehalten? |
| Dateien | Stimmt jede gelistete Datei mit ihrer Prüfsumme im Manifest überein? |
| Zeitstempel | Ist ein Token gültig und von einem vertrauenswürdigen Dienst signiert? |

Code 0 bedeutet passende Dateien und mindestens einen vertrauenswürdigen
Zeitstempel. Erfassungsprobleme können trotzdem vorliegen. Code 10 meldet
veränderte oder ungültige Inhalte und nennt die betroffenen Pfade. Code 11
bedeutet, dass kein vertrauenswürdiger Zeitstempel vorliegt, etwa bei einem
Paket ohne Zeitstempel oder mit dem lokalen Demo-Token. Das Original bleibt
unverändert. Erfasste Dateien werden nie bearbeitet, damit eine Prüfung
besteht.

Ohne Sealjar stehen in `VERIFY.txt` und in der
[Prüfanleitung](../verify.md#without-installing-sealjar) ein Python-3-Skript
für die Dateiprüfung und OpenSSL-Befehle für den Zeitstempel. Die
Wurzelzertifikate müssen aus einer vertrauenswürdigen Quelle stammen. Mit
`-attime` prüft OpenSSL die Zertifikate zum Zeitpunkt `genTime` des Tokens. Die
reine Textanzeige eines Tokens prüft keine Signatur.

Bekommt die Demo keinen Online-Zeitstempel, nutzt sie einen lokalen Token mit
dem Hinweis „Demo timestamp, not trusted“. Die Prüfung endet dann mit Code 11.
Der Signaturschlüssel ist öffentlich, deshalb belegt dieser Token keinen
Zeitpunkt, auch wenn ein ausdrücklich angegebenes Wurzelzertifikat ihm
vertraut.

## Was sich daraus ableiten lässt

Passende Prüfsummen zeigen, dass die gelisteten Dateien zum Manifest passen.
Ein Manifest ohne Zeitstempel können sie allein nicht absichern, denn Dateien
und Prüfsummen könnten gemeinsam ersetzt worden sein. Ein vertrauenswürdiger
Zeitstempel zeigt, dass die versiegelten Dateien spätestens zu diesem Zeitpunkt
vorlagen. Wann die Erfassung stattfand, prüft er nicht.

Als Vertrauensanker dienen die Wurzelzertifikate des Systems, das mitgelieferte
FreeTSA-Root oder separat beschaffte Wurzelzertifikate, die bei der Prüfung
angegeben werden. Zertifikate, die nur im Paket mitkommen, begründen kein
Vertrauen. Eine Offline-Prüfung erkennt spätere Widerrufe nicht. Sealjar v0.1
bewertet keine eIDAS-Qualifikation und bezeichnet keinen Zeitstempel als
qualifiziert.

Das Paket zeigt nicht, ob die Inhalte wahr oder vollständig sind. Es zeigt auch
nicht, ob die erfassende Person ehrlich gehandelt hat oder ob die gespeicherten
Daten wirklich vom Server stammen. Es belegt weder Lieferung noch Abnahme, und
es zeigt nicht, ob der erfasste Code die Website erzeugt hat. Eine rechtliche
Wirkung oder die Zulässigkeit als Beweismittel wird nicht zugesagt. Bei
wichtigen Streitfällen empfiehlt sich eine neutrale Begleitung der Erfassung,
etwa durch einen Sachverständigen oder Notar.

Bei der Erfassung wurden die Zielseiten mit allem geladen, was sie nachladen.
Dazu kamen DNS-Anfragen und die Anfrage an den Zeitstempeldienst, der nur einen
Hash und übliche Netzwerkmetadaten erhält. Maskierung und Ausschlüsse entfernen
nicht alle privaten Inhalte. Solche Inhalte können in Bildschirmfotos und HTML
stehen, ebenso im Code und in der Git-Historie. Vor dem Weiterleiten gehört das
Paket durchgesehen und der Empfängerkreis geklärt.
