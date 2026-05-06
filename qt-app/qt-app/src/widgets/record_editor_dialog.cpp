#include "record_editor_dialog.h"

#include <QFormLayout>
#include <QVBoxLayout>
#include <QHBoxLayout>
#include <QLabel>
#include <QPushButton>
#include <QLineEdit>
#include <QTextEdit>
#include <QComboBox>
#include <QDateEdit>
#include <QAbstractSpinBox>
#include <QRegularExpression>
#include <QDate>

static QString valueToString(const QJsonValue& v)
{
    if (v.isNull() || v.isUndefined()) return QString();
    if (v.isString()) return v.toString();
    if (v.isDouble()) {
        const double d = v.toDouble();
        if (qFuzzyCompare(d + 1.0, 1.0))
            return QString::number(static_cast<qint64>(d));
        return QString::number(d, 'f', 3).remove(QRegularExpression("\\.?0+$"));
    }
    if (v.isBool()) return v.toBool() ? "да" : "нет";
    return {};
}

RecordEditorDialog::RecordEditorDialog(const TableDef& def,
                                       const QJsonObject& existing,
                                       LookupStore* lookup,
                                       QWidget* parent)
    : QDialog(parent),
    m_def(def),
    m_existing(existing),
    m_lookup(lookup)
{
    setWindowTitle("Редактирование: " + def.title);
    resize(940, 720);

    auto* root = new QVBoxLayout(this);
    root->setContentsMargins(16, 16, 16, 16);
    root->setSpacing(12);

    auto* head = new QLabel(
        "<div style='font-size:18px;font-weight:700;'>Редактирование записи</div>"
        "<div style='color:#666;margin-top:4px;'>Поля со звёздочкой обязательны. "
        "Поля, подтянутые из справочников, отображаются только для просмотра.</div>",
        this);
    root->addWidget(head);

    auto* form = new QFormLayout();
    form->setLabelAlignment(Qt::AlignLeft);
    form->setFormAlignment(Qt::AlignTop);
    form->setHorizontalSpacing(14);
    form->setVerticalSpacing(10);

    for (const auto& field : m_def.fields) {
        QWidget* editor = createEditor(field);
        m_editors[field.key] = editor;

        if (m_existing.contains(field.key))
            setEditorValue(editor, field, m_existing.value(field.key));

        if (field.visibleInEditor) {
            form->addRow(field.label + (field.required ? " *" : ""), editor);
        } else if (editor) {
            editor->setVisible(false);
        }
    }

    root->addLayout(form);
    wireDerivedFields();

    auto* buttons = new QHBoxLayout();
    buttons->addStretch();
    auto* saveBtn = new QPushButton("Сохранить", this);
    auto* cancelBtn = new QPushButton("Отмена", this);
    buttons->addWidget(saveBtn);
    buttons->addWidget(cancelBtn);
    root->addLayout(buttons);

    connect(saveBtn, &QPushButton::clicked, this, &QDialog::accept);
    connect(cancelBtn, &QPushButton::clicked, this, &QDialog::reject);

    setStyleSheet(R"(
        QDialog { background: #ffffff; }
        QLabel { color: #111; }
        QLineEdit, QTextEdit, QComboBox, QDateEdit {
            min-height: 32px;
            border: 1px solid #cfd6df;
            border-radius: 10px;
            padding: 6px 10px;
            background: #ffffff;
        }
        QLineEdit:read-only, QTextEdit:read-only, QDateEdit:read-only {
            background: #f4f6f8;
            color: #555;
        }
        QPushButton {
            min-height: 36px;
            padding: 8px 16px;
            border-radius: 10px;
            border: 1px solid #cfd6df;
            background: #111827;
            color: #ffffff;
        }
        QPushButton:hover { background: #1f2937; }
    )");
}

QWidget* RecordEditorDialog::createEditor(const FieldDef& field)
{
    auto makeCombo = [&](const QStringList& texts, const QStringList& data = {}) -> QComboBox* {
        auto* cb = new QComboBox(this);
        if (!data.isEmpty() && data.size() == texts.size()) {
            for (int i = 0; i < texts.size(); ++i)
                cb->addItem(texts[i], data[i]);
        } else {
            for (const auto& t : texts)
                cb->addItem(t, t);
        }
        return cb;
    };

    QWidget* w = nullptr;

    const bool isMachinesTable = (m_def.listAction == "machines.list");
    const bool isMaterialsTable = (m_def.listAction == "materials.list");
    const bool isUnitsTable = (m_def.listAction == "units.list");
    const bool isBrigadesTable = (m_def.listAction == "brigades.list");

    if (field.key == "machine_id" && !isMachinesTable && m_lookup) {
        const auto opts = m_lookup->machineOptions();
        QStringList texts;
        QStringList values;
        for (const auto& o : opts) {
            texts << o.text;
            values << o.value;
        }
        w = makeCombo(texts, values);
    } else if (field.key == "brigade_number" && !isBrigadesTable && m_lookup) {
        const auto opts = m_lookup->brigadeOptions();
        QStringList texts, values;
        for (const auto& o : opts) {
            texts << o.text;
            values << o.value;
        }
        w = makeCombo(texts, values);
    } else if (field.key == "material_code" && !isMaterialsTable && m_lookup) {
        const auto opts = m_lookup->materialOptions();
        QStringList texts, values;
        for (const auto& o : opts) {
            texts << o.text;
            values << o.value;
        }
        w = makeCombo(texts, values);
    } else if (field.key == "unit_symbol" && isMaterialsTable && m_lookup) {
        const auto opts = m_lookup->unitOptions();
        QStringList texts, values;
        for (const auto& o : opts) {
            texts << o.text;
            values << o.value;
        }
        w = makeCombo(texts, values);
    } else if (!field.options.isEmpty()) {
        QStringList texts = field.options;
        QStringList values = field.optionValues.isEmpty() ? field.options : field.optionValues;
        w = makeCombo(texts, values);
    } else if (field.kind == FieldKind::Date) {
        auto* de = new QDateEdit(this);
        de->setCalendarPopup(true);
        de->setDisplayFormat("dd-MM-yyyy");
        de->setDate(QDate::currentDate());
        w = de;
    } else if (field.kind == FieldKind::Multiline) {
        auto* te = new QTextEdit(this);
        te->setMinimumHeight(86);
        w = te;
    } else {
        auto* le = new QLineEdit(this);
        if (!field.placeholder.isEmpty())
            le->setPlaceholderText(field.placeholder);
        w = le;
    }

    if (!field.editable && w) {
        if (auto* le = qobject_cast<QLineEdit*>(w)) {
            le->setReadOnly(true);
        } else if (auto* te = qobject_cast<QTextEdit*>(w)) {
            te->setReadOnly(true);
        } else if (auto* de = qobject_cast<QDateEdit*>(w)) {
            de->setReadOnly(true);
            de->setButtonSymbols(QAbstractSpinBox::NoButtons);
        } else if (auto* cb = qobject_cast<QComboBox*>(w)) {
            cb->setEnabled(false);
        }
    }

    return w;
}

void RecordEditorDialog::setEditorValue(QWidget* editor, const FieldDef&, const QJsonValue& value)
{
    const QString s = valueToString(value);

    if (auto* cb = qobject_cast<QComboBox*>(editor)) {
        int idx = cb->findData(s);
        if (idx < 0)
            idx = cb->findText(s);
        if (idx >= 0)
            cb->setCurrentIndex(idx);
        return;
    }

    if (auto* de = qobject_cast<QDateEdit*>(editor)) {
        QDate d = QDate::fromString(s, "yyyy-MM-dd");
        if (!d.isValid())
            d = QDate::fromString(s, "dd-MM-yyyy");
        if (d.isValid())
            de->setDate(d);
        return;
    }

    if (auto* te = qobject_cast<QTextEdit*>(editor)) {
        te->setPlainText(s);
        return;
    }

    if (auto* le = qobject_cast<QLineEdit*>(editor)) {
        le->setText(s);
        return;
    }
}

QString RecordEditorDialog::readEditor(QWidget* editor, const FieldDef& field) const
{
    Q_UNUSED(field);

    if (!editor) return {};

    if (auto* cb = qobject_cast<QComboBox*>(editor)) {
        const QVariant d = cb->currentData();
        if (d.isValid() && !d.toString().trimmed().isEmpty())
            return d.toString().trimmed();
        return cb->currentText().trimmed();
    }

    if (auto* de = qobject_cast<QDateEdit*>(editor))
        return de->date().toString("yyyy-MM-dd");

    if (auto* te = qobject_cast<QTextEdit*>(editor))
        return te->toPlainText().trimmed();

    if (auto* le = qobject_cast<QLineEdit*>(editor))
        return le->text().trimmed();

    return {};
}

QWidget* RecordEditorDialog::editorFor(const QString& key) const
{
    return m_editors.value(key, nullptr);
}

void RecordEditorDialog::setFieldText(const QString& key, const QString& value)
{
    QWidget* w = editorFor(key);
    if (!w) return;

    if (auto* cb = qobject_cast<QComboBox*>(w)) {
        const int idx = cb->findData(value);
        if (idx >= 0) {
            cb->setCurrentIndex(idx);
            return;
        }
        const int idx2 = cb->findText(value);
        if (idx2 >= 0)
            cb->setCurrentIndex(idx2);
        return;
    }

    if (auto* le = qobject_cast<QLineEdit*>(w)) {
        le->setText(value);
        return;
    }

    if (auto* te = qobject_cast<QTextEdit*>(w)) {
        te->setPlainText(value);
        return;
    }

    if (auto* de = qobject_cast<QDateEdit*>(w)) {
        QDate d = QDate::fromString(value, "yyyy-MM-dd");
        if (!d.isValid())
            d = QDate::fromString(value, "dd-MM-yyyy");
        if (d.isValid())
            de->setDate(d);
        return;
    }
}

void RecordEditorDialog::wireDerivedFields()
{
    const bool isMachinesTable = (m_def.listAction == "machines.list");
    const bool isMaterialsTable = (m_def.listAction == "materials.list");

    auto syncMachineModel = [this]() {
        QWidget* machineW = editorFor("machine_id");
        if (!machineW || !m_lookup) return;

        QString machineId;
        if (auto* cb = qobject_cast<QComboBox*>(machineW))
            machineId = cb->currentData().toString().trimmed();
        else if (auto* le = qobject_cast<QLineEdit*>(machineW))
            machineId = le->text().trimmed();

        const QString model = m_lookup->machineModel(machineId);
        setFieldText("model", model);
    };

    auto syncMaterialDerived = [this]() {
        QWidget* codeW = editorFor("material_code");
        if (!codeW || !m_lookup) return;

        QString code;
        if (auto* cb = qobject_cast<QComboBox*>(codeW))
            code = cb->currentData().toString().trimmed();
        else if (auto* le = qobject_cast<QLineEdit*>(codeW))
            code = le->text().trimmed();

        setFieldText("material_name", m_lookup->materialName(code));
        setFieldText("unit_symbol", m_lookup->materialUnitSymbol(code));
    };

    auto syncUnitHidden = [this]() {
        QWidget* unitW = editorFor("unit_symbol");
        if (!unitW || !m_lookup) return;

        QString sym;
        if (auto* cb = qobject_cast<QComboBox*>(unitW))
            sym = cb->currentText().trimmed();
        else if (auto* le = qobject_cast<QLineEdit*>(unitW))
            sym = le->text().trimmed();

        setFieldText("unit_id", m_lookup->unitIdBySymbol(sym));
    };

    if (QComboBox* machineCb = qobject_cast<QComboBox*>(editorFor("machine_id"))) {
        if (!isMachinesTable && m_lookup) {
            connect(machineCb, &QComboBox::currentIndexChanged, this, [syncMachineModel]() {
                syncMachineModel();
            });
            syncMachineModel();
        }
    } else {
        syncMachineModel();
    }

    if (QComboBox* materialCb = qobject_cast<QComboBox*>(editorFor("material_code"))) {
        if (m_lookup) {
            connect(materialCb, &QComboBox::currentIndexChanged, this, [syncMaterialDerived]() {
                syncMaterialDerived();
            });
            syncMaterialDerived();
        }
    } else {
        syncMaterialDerived();
    }

    if (QComboBox* unitCb = qobject_cast<QComboBox*>(editorFor("unit_symbol"))) {
        if (isMaterialsTable && m_lookup) {
            connect(unitCb, &QComboBox::currentIndexChanged, this, [syncUnitHidden]() {
                syncUnitHidden();
            });
            syncUnitHidden();
        }
    } else {
        syncUnitHidden();
    }
}

QJsonObject RecordEditorDialog::data() const
{
    QJsonObject obj;

    for (const auto& field : m_def.fields) {
        QWidget* editor = m_editors.value(field.key, nullptr);
        if (!editor) continue;

        const QString text = readEditor(editor, field);

        if (text.isEmpty()) {
            if (field.required)
                obj[field.key] = QString();
            continue;
        }

        switch (field.kind) {
        case FieldKind::Int:
            obj[field.key] = text.toInt();
            break;
        case FieldKind::Double:
            obj[field.key] = text.toDouble();
            break;
        case FieldKind::Date:
            obj[field.key] = text;
            break;
        default:
            obj[field.key] = text;
            break;
        }
    }

    return obj;
}
