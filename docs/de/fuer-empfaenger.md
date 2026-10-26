# Ein erhaltenes Paket lesen und prüfen

Das ZIP enthält die vollständige Erfassung. Der zugehörige Leseordner enthält
Berichte und Bilder. `report/report.de.html` und das Bericht-PDF sind ohne
Sealjar-Installation lesbar. Der HTML-Bericht nutzt relative Bildverweise, daher
muss der Leseordner zusammenbleiben. Das PDF ist eine eigenständige Lesekopie.

Zuerst lohnt sich der Blick auf die Zusammenfassung. Stimmen URLs, Repository und
Commit mit dem erwarteten Umfang überein? Welche Dateien wurden ausgeschlossen?
Fehlende Ressourcen oder fehlgeschlagene Seiten können Lücken hinterlassen. Die
Bildschirmfotos zeigen die erfassten Ansichten, keine vollständige Prüfung aller
Funktionen einer Website.

Der Bericht verweist für Zeitstempel auf `SEAL.html` und die Paketprüfung. Die
Siegelseite entsteht nach dem Manifest und gehört selbst nicht zu dessen Dateiliste.
Ihr Vertrauensstatus stammt vom damaligen Prüflauf. `verify` prüft die Token direkt.
Beim Berichtsexport wird die Siegelseite aus geprüften Daten neu erstellt.

## Drei getrennte Ergebnisse

Nach der [Installation](../quickstart.md#install) prüft dieser Befehl das ZIP. Der
Beispielname wird durch den tatsächlichen Pfad ersetzt:

```sh
sealjar verify paket.sealjar.zip
```

| Ergebnis | Bedeutung |
| --- | --- |
| Erfassung | Wurde die Erfassung abgeschlossen? Welche Probleme wurden dabei festgehalten? |
| Dateien | Stimmen die Dateien mit den Prüfsummen (SHA-256) im Manifest überein? |
| Zeitstempel | Ist der Token gültig und vertrauenswürdig? Passt er zum Manifest? |

Code 0 bedeutet passende Dateien und mindestens einen vertrauenswürdigen
Zeitstempel. Erfassungsprobleme können dennoch vorliegen. Code 10 meldet veränderte
oder ungültige Inhalte mit betroffenen Pfaden. Code 11 bedeutet, dass kein
vertrauenswürdiger Zeitstempel vorliegt, etwa bei einem Paket ohne Zeitstempel oder
mit dem ausdrücklich nicht vertrauenswürdigen Demo-Zeitstempel. Das Original bleibt
erhalten. Erfasste Dateien werden nicht zur Reparatur eines Prüfergebnisses
verändert.

Ohne Sealjar stehen in `VERIFY.txt` und der
[Prüfanleitung](../verify.md#without-installing-sealjar) ein Python-3-Skript für die
Dateiprüfung und OpenSSL-Befehle für den Zeitstempel. Vertrauensanker müssen
unabhängig bezogen werden. Mit `-attime` erfolgt die Zertifikatsprüfung zum Zeitpunkt `genTime` des Tokens. Die reine
Textanzeige eines Tokens prüft keine Signatur.

Wenn die Demo keinen Online-Zeitstempel erhält, nutzt sie einen lokalen Token mit
dem Hinweis „Demo timestamp, not trusted“. Die Prüfung endet dann mit Code 11. Der
Signaturschlüssel ist öffentlich. Auch wenn ein ausdrücklich angegebenes
Wurzelzertifikat dem Token Vertrauen gewährt, liefert er keinen unabhängigen
Zeitnachweis.

## Was sich daraus ableiten lässt

Passende Prüfsummen zeigen, dass die gelisteten Dateien mit dem Manifest
übereinstimmen. Allein können sie ein Manifest ohne Zeitstempel nicht
authentifizieren: Dateien und Prüfsummen könnten gemeinsam ersetzt worden sein. Ein
vertrauenswürdiger Zeitstempel zeigt, dass die versiegelten Dateien spätestens zu
diesem Zeitpunkt vorlagen. Wann die Erfassung stattfand, wird dadurch nicht
unabhängig geprüft.

Als Vertrauensanker dienen System-Wurzelzertifikate, das eingebettete FreeTSA-Root
oder separat beschaffte Wurzelzertifikate, die bei der Prüfung angegeben werden.
Zertifikate, die lediglich im Paket mitgeliefert werden, begründen kein Vertrauen.
Offline-Prüfungen erkennen spätere Zertifikatswiderrufe nicht. Sealjar v0.1
bewertet keine eIDAS-Qualifikation und bezeichnet Zeitstempel nicht als
qualifiziert.

Das Paket zeigt nicht, ob die Inhalte wahr oder vollständig sind. Es zeigt auch
nicht, ob die erfassende Person ehrlich gehandelt hat oder die gespeicherten Daten
vom Server stammen. Es belegt weder Lieferung noch Abnahme. Ob der erfasste Code
die Website erzeugt hat, bleibt ebenfalls offen. Eine rechtliche Wirkung oder
Zulässigkeit als Beweismittel wird nicht zugesagt. Bei wichtigen Streitfällen
empfiehlt sich eine neutrale Begleitung der Erfassung, etwa durch einen
Sachverständigen oder Notar.

Bei der Erfassung werden Zielseiten und deren Ressourcen geladen. Hinzu kommen
Anfragen an DNS-Server und Zeitstempeldienste. Der Zeitstempeldienst erhält einen
Hash und übliche Netzwerkmetadaten. Zeitstempelanfragen beachten Standard-Proxys;
die Verbindungsproben von `doctor` tun das nicht. Maskierung und Ausschlüsse
entfernen nicht alle privaten Inhalte. Solche Inhalte können weiterhin in
Bildschirmfotos und HTML stehen, ebenso im Code und in der Git-Historie. Vor dem
Weiterleiten sollte das Paket auf solche Inhalte geprüft und der Empfängerkreis
geklärt werden.
