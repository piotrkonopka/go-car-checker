import os
import pandas as pd
import matplotlib.pyplot as plt

print("🔍 Rozpoczynam generowanie wykresów...")

# Tworzymy katalog na wykresy
charts_dir = "charts"
os.makedirs(charts_dir, exist_ok=True)
print(f"📁 Upewniono się, że katalog '{charts_dir}' istnieje.")

# Lista plików CSV z danymi (wszystkie w katalogu 'data')
csv_files = [
    "Peugeot_Rifter_olx.csv",
    "Citroen_Berlingo_olx.csv",
    "Toyota_Proace_City_olx.csv",
    "Peugeot_Rifter_otomoto.csv",
    "Citroen_Berlingo_otomoto.csv",
    "Toyota_Proace_City_otomoto.csv"
]

for file in csv_files:
    file_path = os.path.join("data", file)
    print(f"\n📄 Sprawdzam plik: {file_path}")

    if not os.path.exists(file_path):
        print(f"⚠️  Plik {file_path} nie istnieje — pomijam.")
        continue

    try:
        df = pd.read_csv(file_path, names=["date", "segment", "avg_price"])
        df["date"] = pd.to_datetime(df["date"])
        print(f"✅ Wczytano dane ({len(df)} rekordów)")

        if df.empty:
            print("⚠️  Dane są puste — pomijam.")
            continue

        for seg in df["segment"].unique():
            sub = df[df["segment"] == seg]
            if sub.empty:
                print(f"⚠️  Brak danych dla segmentu {seg}")
                continue

            plt.plot(sub["date"], sub["avg_price"], label=f"Segment {seg}")

        title = file.replace(".csv", "").replace("_", " ")
        plt.title(title)
        plt.xlabel("Data")
        plt.ylabel("Średnia cena (zł)")
        plt.legend()
        plt.grid(True)
        plt.xticks(rotation=45)
        plt.tight_layout()

        output_path = os.path.join(charts_dir, file.replace('.csv', '') + ".png")
        plt.savefig(output_path)
        plt.close()

        print(f"📊 Wykres zapisano jako {output_path}")

    except Exception as e:
        print(f"❌ Błąd podczas przetwarzania {file_path}: {e}")

print("\n✅ Zakończono generowanie wykresów.")